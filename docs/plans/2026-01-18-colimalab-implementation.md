# CollimaLab Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Deploy a Colima-based container environment with NFS-backed storage on Unraid, k3d for Kubernetes, and Ollama with Mistral Small 3.

**Architecture:** Pulumi (Go) orchestrates: (1) SSH to Unraid to configure NFS export, (2) local commands to mount NFS, install Colima, create k3d cluster, and install Ollama.

**Tech Stack:** Go, Pulumi, SSH, NFS, Colima, Docker, k3d, Ollama

---

## Task 1: Initialize Pulumi Project

**Files:**
- Create: `Pulumi.yaml`
- Create: `Pulumi.dev.yaml`
- Create: `go.mod`
- Create: `main.go`

**Step 1: Create Pulumi.yaml**

```yaml
name: colimalab
runtime: go
description: Colima + k3d + Ollama dev environment with NFS storage
```

**Step 2: Create Pulumi.dev.yaml**

```yaml
config:
  colimalab:unraid-host: "10.0.0.10"
  colimalab:unraid-user: "root"
  colimalab:nfs-path: "/mnt/store/colimalab"
  colimalab:nfs-mount: "/Volumes/CollimaLab"
  colimalab:colima-cpu: "10"
  colimalab:colima-memory: "12"
  colimalab:colima-disk: "10"
  colimalab:ollama-model: "mistral-small:24b-instruct-2501-q4_K_M"
```

**Step 3: Initialize Go module**

Run: `go mod init github.com/jamesainslie/CollimaLab`

**Step 4: Create main.go skeleton**

```go
package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// Components will be added here
		return nil
	})
}
```

**Step 5: Add Pulumi SDK dependency**

Run: `go get github.com/pulumi/pulumi/sdk/v3/go/pulumi`

**Step 6: Verify build**

Run: `go build -o /dev/null .`
Expected: No errors

**Step 7: Commit**

```bash
git add .
git commit -m "feat: initialize Pulumi Go project"
```

---

## Task 2: Create SSH Utility Package

**Files:**
- Create: `pkg/util/ssh.go`

**Step 1: Create pkg/util directory**

Run: `mkdir -p pkg/util`

**Step 2: Write SSH helper**

```go
package util

import (
	"fmt"
	"os/exec"
	"strings"
)

// SSHConfig holds SSH connection details
type SSHConfig struct {
	Host string
	User string
}

// RunSSH executes a command on remote host via SSH
func RunSSH(cfg SSHConfig, command string) (string, error) {
	target := fmt.Sprintf("%s@%s", cfg.User, cfg.Host)
	cmd := exec.Command("ssh", "-o", "BatchMode=yes", "-o", "StrictHostKeyChecking=accept-new", target, command)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("ssh command failed: %w\noutput: %s", err, output)
	}
	return strings.TrimSpace(string(output)), nil
}

// TestSSH verifies SSH connectivity
func TestSSH(cfg SSHConfig) error {
	_, err := RunSSH(cfg, "echo ok")
	return err
}
```

**Step 3: Verify compilation**

Run: `go build ./pkg/util/`
Expected: No errors

**Step 4: Commit**

```bash
git add pkg/
git commit -m "feat: add SSH utility package"
```

---

## Task 3: Create Local Exec Utility Package

**Files:**
- Modify: `pkg/util/exec.go`

**Step 1: Write local exec helper**

```go
package util

import (
	"fmt"
	"os/exec"
	"strings"
)

// RunLocal executes a local command
func RunLocal(command string, args ...string) (string, error) {
	cmd := exec.Command(command, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("command failed: %w\noutput: %s", err, output)
	}
	return strings.TrimSpace(string(output)), nil
}

// RunLocalShell executes a command via shell
func RunLocalShell(command string) (string, error) {
	cmd := exec.Command("bash", "-c", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("shell command failed: %w\noutput: %s", err, output)
	}
	return strings.TrimSpace(string(output)), nil
}

// CommandExists checks if a command is available
func CommandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
```

**Step 2: Verify compilation**

Run: `go build ./pkg/util/`
Expected: No errors

**Step 3: Commit**

```bash
git add pkg/util/exec.go
git commit -m "feat: add local exec utility package"
```

---

## Task 4: Implement Unraid NFS Module

**Files:**
- Create: `pkg/unraid/nfs.go`

**Step 1: Create pkg/unraid directory**

Run: `mkdir -p pkg/unraid`

**Step 2: Write NFS export component**

```go
package unraid

import (
	"fmt"
	"strings"

	"github.com/jamesainslie/CollimaLab/pkg/util"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// NFSExportArgs contains configuration for NFS export
type NFSExportArgs struct {
	Host       string
	User       string
	ExportPath string
	Network    string // e.g., "10.0.0.0/24"
}

// NFSExport represents an NFS export on Unraid
type NFSExport struct {
	pulumi.ResourceState

	ExportPath pulumi.StringOutput `pulumi:"exportPath"`
}

// NewNFSExport creates and configures an NFS export on Unraid
func NewNFSExport(ctx *pulumi.Context, name string, args *NFSExportArgs, opts ...pulumi.ResourceOption) (*NFSExport, error) {
	component := &NFSExport{}
	err := ctx.RegisterComponentResource("colimalab:unraid:NFSExport", name, component, opts...)
	if err != nil {
		return nil, err
	}

	sshCfg := util.SSHConfig{Host: args.Host, User: args.User}

	// Test SSH connectivity
	if err := util.TestSSH(sshCfg); err != nil {
		return nil, fmt.Errorf("cannot connect to Unraid: %w", err)
	}

	// Create export directory
	mkdirCmd := fmt.Sprintf("mkdir -p %s/docker && chmod 777 %s", args.ExportPath, args.ExportPath)
	if _, err := util.RunSSH(sshCfg, mkdirCmd); err != nil {
		return nil, fmt.Errorf("failed to create export directory: %w", err)
	}

	// Check if export already exists
	checkCmd := fmt.Sprintf("grep -q '%s' /etc/exports 2>/dev/null && echo exists || echo missing", args.ExportPath)
	status, _ := util.RunSSH(sshCfg, checkCmd)

	if strings.TrimSpace(status) == "missing" {
		// Add NFS export
		exportLine := fmt.Sprintf("%s %s(rw,async,no_subtree_check,no_root_squash,all_squash,anonuid=0,anongid=0)",
			args.ExportPath, args.Network)
		addExportCmd := fmt.Sprintf("echo '%s' >> /etc/exports", exportLine)
		if _, err := util.RunSSH(sshCfg, addExportCmd); err != nil {
			return nil, fmt.Errorf("failed to add NFS export: %w", err)
		}

		// Reload NFS exports
		if _, err := util.RunSSH(sshCfg, "exportfs -ra"); err != nil {
			return nil, fmt.Errorf("failed to reload NFS exports: %w", err)
		}
	}

	component.ExportPath = pulumi.String(args.ExportPath).ToStringOutput()

	ctx.RegisterResourceOutputs(component, pulumi.Map{
		"exportPath": component.ExportPath,
	})

	return component, nil
}
```

**Step 3: Verify compilation**

Run: `go build ./pkg/unraid/`
Expected: No errors

**Step 4: Commit**

```bash
git add pkg/unraid/
git commit -m "feat: add Unraid NFS export component"
```

---

## Task 5: Implement Mac NFS Mount Module

**Files:**
- Create: `pkg/mac/nfs.go`

**Step 1: Create pkg/mac directory**

Run: `mkdir -p pkg/mac`

**Step 2: Write NFS mount component**

```go
package mac

import (
	"fmt"
	"os"
	"strings"

	"github.com/jamesainslie/CollimaLab/pkg/util"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// NFSMountArgs contains configuration for NFS mount
type NFSMountArgs struct {
	ServerHost string
	ServerPath string
	MountPoint string
}

// NFSMount represents an NFS mount on Mac
type NFSMount struct {
	pulumi.ResourceState

	MountPoint pulumi.StringOutput `pulumi:"mountPoint"`
}

// NewNFSMount creates and mounts an NFS share on Mac
func NewNFSMount(ctx *pulumi.Context, name string, args *NFSMountArgs, opts ...pulumi.ResourceOption) (*NFSMount, error) {
	component := &NFSMount{}
	err := ctx.RegisterComponentResource("colimalab:mac:NFSMount", name, component, opts...)
	if err != nil {
		return nil, err
	}

	// Create mount point directory
	if err := os.MkdirAll(args.MountPoint, 0755); err != nil {
		return nil, fmt.Errorf("failed to create mount point: %w", err)
	}

	// Check if already mounted
	checkCmd := fmt.Sprintf("mount | grep -q '%s' && echo mounted || echo unmounted", args.MountPoint)
	status, _ := util.RunLocalShell(checkCmd)

	if strings.TrimSpace(status) == "unmounted" {
		// Mount NFS share
		nfsPath := fmt.Sprintf("%s:%s", args.ServerHost, args.ServerPath)
		mountCmd := fmt.Sprintf("mount -t nfs -o resvport,rw,noatime %s %s", nfsPath, args.MountPoint)
		if _, err := util.RunLocalShell(mountCmd); err != nil {
			return nil, fmt.Errorf("failed to mount NFS: %w", err)
		}
	}

	// Add to /etc/auto_nfs for persistence (optional - needs sudo)
	// For now, we'll just verify the mount works

	component.MountPoint = pulumi.String(args.MountPoint).ToStringOutput()

	ctx.RegisterResourceOutputs(component, pulumi.Map{
		"mountPoint": component.MountPoint,
	})

	return component, nil
}
```

**Step 3: Verify compilation**

Run: `go build ./pkg/mac/`
Expected: No errors

**Step 4: Commit**

```bash
git add pkg/mac/
git commit -m "feat: add Mac NFS mount component"
```

---

## Task 6: Implement Colima Module

**Files:**
- Create: `pkg/mac/colima.go`

**Step 1: Write Colima component**

```go
package mac

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jamesainslie/CollimaLab/pkg/util"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// ColimaArgs contains configuration for Colima
type ColimaArgs struct {
	CPU        int
	Memory     int
	Disk       int
	NFSMount   string // Mount point to pass through to VM
	DockerRoot string // Docker data-root path (on NFS)
}

// Colima represents a Colima installation
type Colima struct {
	pulumi.ResourceState

	Status pulumi.StringOutput `pulumi:"status"`
}

// NewColima installs and configures Colima
func NewColima(ctx *pulumi.Context, name string, args *ColimaArgs, opts ...pulumi.ResourceOption) (*Colima, error) {
	component := &Colima{}
	err := ctx.RegisterComponentResource("colimalab:mac:Colima", name, component, opts...)
	if err != nil {
		return nil, err
	}

	homeDir, _ := os.UserHomeDir()

	// Stop existing Colima if running
	util.RunLocal("colima", "stop")

	// Backup existing config
	colimaDir := filepath.Join(homeDir, ".colima")
	if _, err := os.Stat(colimaDir); err == nil {
		backupDir := colimaDir + ".bak"
		os.RemoveAll(backupDir)
		os.Rename(colimaDir, backupDir)
	}

	// Uninstall existing Colima
	util.RunLocal("brew", "uninstall", "colima")
	util.RunLocal("brew", "uninstall", "docker")

	// Clean up Lima VM
	limaDir := filepath.Join(homeDir, ".lima", "colima")
	os.RemoveAll(limaDir)

	// Install fresh Colima and Docker CLI
	if _, err := util.RunLocal("brew", "install", "colima"); err != nil {
		return nil, fmt.Errorf("failed to install colima: %w", err)
	}
	if _, err := util.RunLocal("brew", "install", "docker"); err != nil {
		return nil, fmt.Errorf("failed to install docker: %w", err)
	}

	// Start Colima with configuration
	startCmd := fmt.Sprintf(
		"colima start --cpu %d --memory %d --disk %d --vm-type vz --mount-type virtiofs --mount %s:w",
		args.CPU, args.Memory, args.Disk, args.NFSMount,
	)
	if _, err := util.RunLocalShell(startCmd); err != nil {
		return nil, fmt.Errorf("failed to start colima: %w", err)
	}

	// Configure Docker data-root
	// Create daemon.json in Colima VM
	daemonConfig := fmt.Sprintf(`{"data-root": "%s"}`, args.DockerRoot)
	configCmd := fmt.Sprintf(`colima ssh -- 'sudo mkdir -p /etc/docker && echo '\''%s'\'' | sudo tee /etc/docker/daemon.json && sudo systemctl restart docker'`, daemonConfig)
	if _, err := util.RunLocalShell(configCmd); err != nil {
		return nil, fmt.Errorf("failed to configure docker data-root: %w", err)
	}

	// Verify Docker works
	output, err := util.RunLocal("docker", "info")
	if err != nil {
		return nil, fmt.Errorf("docker verification failed: %w", err)
	}

	if !strings.Contains(output, args.DockerRoot) {
		ctx.Log.Warn("Docker data-root may not be configured correctly", nil)
	}

	component.Status = pulumi.String("running").ToStringOutput()

	ctx.RegisterResourceOutputs(component, pulumi.Map{
		"status": component.Status,
	})

	return component, nil
}
```

**Step 2: Verify compilation**

Run: `go build ./pkg/mac/`
Expected: No errors

**Step 3: Commit**

```bash
git add pkg/mac/colima.go
git commit -m "feat: add Colima install and config component"
```

---

## Task 7: Implement k3d Module

**Files:**
- Create: `pkg/mac/k3d.go`

**Step 1: Write k3d component**

```go
package mac

import (
	"fmt"
	"strings"

	"github.com/jamesainslie/CollimaLab/pkg/util"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// K3dClusterArgs contains configuration for k3d cluster
type K3dClusterArgs struct {
	Name    string
	Servers int
	Ports   []string // e.g., ["80:80@loadbalancer", "443:443@loadbalancer"]
}

// K3dCluster represents a k3d cluster
type K3dCluster struct {
	pulumi.ResourceState

	Name   pulumi.StringOutput `pulumi:"name"`
	Status pulumi.StringOutput `pulumi:"status"`
}

// NewK3dCluster creates a k3d cluster
func NewK3dCluster(ctx *pulumi.Context, name string, args *K3dClusterArgs, opts ...pulumi.ResourceOption) (*K3dCluster, error) {
	component := &K3dCluster{}
	err := ctx.RegisterComponentResource("colimalab:mac:K3dCluster", name, component, opts...)
	if err != nil {
		return nil, err
	}

	// Install k3d if not present
	if !util.CommandExists("k3d") {
		if _, err := util.RunLocal("brew", "install", "k3d"); err != nil {
			return nil, fmt.Errorf("failed to install k3d: %w", err)
		}
	}

	// Install kubectl if not present
	if !util.CommandExists("kubectl") {
		if _, err := util.RunLocal("brew", "install", "kubectl"); err != nil {
			return nil, fmt.Errorf("failed to install kubectl: %w", err)
		}
	}

	// Check if cluster already exists
	listOutput, _ := util.RunLocal("k3d", "cluster", "list", "-o", "json")
	clusterExists := strings.Contains(listOutput, fmt.Sprintf(`"name":"%s"`, args.Name))

	if !clusterExists {
		// Build create command
		createArgs := []string{"cluster", "create", args.Name, "--servers", fmt.Sprintf("%d", args.Servers)}
		for _, port := range args.Ports {
			createArgs = append(createArgs, "--port", port)
		}

		if _, err := util.RunLocal("k3d", createArgs...); err != nil {
			return nil, fmt.Errorf("failed to create k3d cluster: %w", err)
		}
	}

	// Verify cluster is running
	output, err := util.RunLocal("kubectl", "get", "nodes")
	if err != nil {
		return nil, fmt.Errorf("kubectl verification failed: %w", err)
	}

	status := "running"
	if !strings.Contains(output, "Ready") {
		status = "not ready"
	}

	component.Name = pulumi.String(args.Name).ToStringOutput()
	component.Status = pulumi.String(status).ToStringOutput()

	ctx.RegisterResourceOutputs(component, pulumi.Map{
		"name":   component.Name,
		"status": component.Status,
	})

	return component, nil
}
```

**Step 2: Verify compilation**

Run: `go build ./pkg/mac/`
Expected: No errors

**Step 3: Commit**

```bash
git add pkg/mac/k3d.go
git commit -m "feat: add k3d cluster component"
```

---

## Task 8: Implement Ollama Module

**Files:**
- Create: `pkg/mac/ollama.go`

**Step 1: Write Ollama component**

```go
package mac

import (
	"fmt"
	"strings"
	"time"

	"github.com/jamesainslie/CollimaLab/pkg/util"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// OllamaArgs contains configuration for Ollama
type OllamaArgs struct {
	Model string // e.g., "mistral-small:24b-instruct-2501-q4_K_M"
}

// Ollama represents an Ollama installation
type Ollama struct {
	pulumi.ResourceState

	Model  pulumi.StringOutput `pulumi:"model"`
	Status pulumi.StringOutput `pulumi:"status"`
}

// NewOllama installs Ollama and pulls the specified model
func NewOllama(ctx *pulumi.Context, name string, args *OllamaArgs, opts ...pulumi.ResourceOption) (*Ollama, error) {
	component := &Ollama{}
	err := ctx.RegisterComponentResource("colimalab:mac:Ollama", name, component, opts...)
	if err != nil {
		return nil, err
	}

	// Install Ollama if not present
	if !util.CommandExists("ollama") {
		if _, err := util.RunLocal("brew", "install", "ollama"); err != nil {
			return nil, fmt.Errorf("failed to install ollama: %w", err)
		}
	}

	// Start Ollama service
	util.RunLocal("brew", "services", "start", "ollama")

	// Wait for Ollama to be ready
	for i := 0; i < 30; i++ {
		output, err := util.RunLocalShell("curl -s http://localhost:11434/api/tags")
		if err == nil && strings.Contains(output, "models") {
			break
		}
		time.Sleep(time.Second)
	}

	// Check if model already exists
	listOutput, _ := util.RunLocalShell("ollama list")
	modelName := strings.Split(args.Model, ":")[0]
	modelExists := strings.Contains(listOutput, modelName)

	if !modelExists {
		ctx.Log.Info(fmt.Sprintf("Pulling model %s (this may take a while)...", args.Model), nil)
		if _, err := util.RunLocal("ollama", "pull", args.Model); err != nil {
			return nil, fmt.Errorf("failed to pull ollama model: %w", err)
		}
	}

	// Verify model is available
	listOutput, _ = util.RunLocalShell("ollama list")
	status := "ready"
	if !strings.Contains(listOutput, modelName) {
		status = "model not found"
	}

	component.Model = pulumi.String(args.Model).ToStringOutput()
	component.Status = pulumi.String(status).ToStringOutput()

	ctx.RegisterResourceOutputs(component, pulumi.Map{
		"model":  component.Model,
		"status": component.Status,
	})

	return component, nil
}
```

**Step 2: Verify compilation**

Run: `go build ./pkg/mac/`
Expected: No errors

**Step 3: Commit**

```bash
git add pkg/mac/ollama.go
git commit -m "feat: add Ollama install component"
```

---

## Task 9: Wire Up Main Entry Point

**Files:**
- Modify: `main.go`

**Step 1: Update main.go with all components**

```go
package main

import (
	"github.com/jamesainslie/CollimaLab/pkg/mac"
	"github.com/jamesainslie/CollimaLab/pkg/unraid"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := config.New(ctx, "colimalab")

		// Get configuration values
		unraidHost := cfg.Require("unraid-host")
		unraidUser := cfg.Require("unraid-user")
		nfsPath := cfg.Require("nfs-path")
		nfsMount := cfg.Require("nfs-mount")
		colimaCPU := cfg.RequireInt("colima-cpu")
		colimaMemory := cfg.RequireInt("colima-memory")
		colimaDisk := cfg.RequireInt("colima-disk")
		ollamaModel := cfg.Require("ollama-model")

		// 1. Configure NFS export on Unraid
		nfsExport, err := unraid.NewNFSExport(ctx, "colimalab-nfs", &unraid.NFSExportArgs{
			Host:       unraidHost,
			User:       unraidUser,
			ExportPath: nfsPath,
			Network:    "10.0.0.0/24",
		})
		if err != nil {
			return err
		}

		// 2. Mount NFS on Mac
		nfsMountRes, err := mac.NewNFSMount(ctx, "colimalab-mount", &mac.NFSMountArgs{
			ServerHost: unraidHost,
			ServerPath: nfsPath,
			MountPoint: nfsMount,
		}, pulumi.DependsOn([]pulumi.Resource{nfsExport}))
		if err != nil {
			return err
		}

		// 3. Install and configure Colima
		colima, err := mac.NewColima(ctx, "colimalab-colima", &mac.ColimaArgs{
			CPU:        colimaCPU,
			Memory:     colimaMemory,
			Disk:       colimaDisk,
			NFSMount:   nfsMount,
			DockerRoot: nfsMount + "/docker",
		}, pulumi.DependsOn([]pulumi.Resource{nfsMountRes}))
		if err != nil {
			return err
		}

		// 4. Create k3d cluster
		k3dCluster, err := mac.NewK3dCluster(ctx, "colimalab-k3d", &mac.K3dClusterArgs{
			Name:    "lab",
			Servers: 1,
			Ports:   []string{"80:80@loadbalancer", "443:443@loadbalancer"},
		}, pulumi.DependsOn([]pulumi.Resource{colima}))
		if err != nil {
			return err
		}

		// 5. Install Ollama with Mistral model
		ollama, err := mac.NewOllama(ctx, "colimalab-ollama", &mac.OllamaArgs{
			Model: ollamaModel,
		}, pulumi.DependsOn([]pulumi.Resource{colima}))
		if err != nil {
			return err
		}

		// Export outputs
		ctx.Export("nfsExportPath", nfsExport.ExportPath)
		ctx.Export("nfsMountPoint", nfsMountRes.MountPoint)
		ctx.Export("colimaStatus", colima.Status)
		ctx.Export("k3dClusterName", k3dCluster.Name)
		ctx.Export("k3dClusterStatus", k3dCluster.Status)
		ctx.Export("ollamaModel", ollama.Model)
		ctx.Export("ollamaStatus", ollama.Status)

		return nil
	})
}
```

**Step 2: Run go mod tidy**

Run: `go mod tidy`

**Step 3: Verify build**

Run: `go build .`
Expected: No errors

**Step 4: Commit**

```bash
git add main.go go.mod go.sum
git commit -m "feat: wire up all components in main.go"
```

---

## Task 10: Initialize Pulumi Stack and Test

**Step 1: Log in to Pulumi (local backend for now)**

Run: `pulumi login --local`

**Step 2: Initialize dev stack**

Run: `pulumi stack init dev`

**Step 3: Preview deployment**

Run: `pulumi preview`
Expected: Shows planned changes for all components

**Step 4: Deploy**

Run: `pulumi up --yes`
Expected: All components deploy successfully

**Step 5: Verify deployment**

Run these verification commands:
```bash
# NFS mount
mount | grep CollimaLab

# Colima
colima status

# Docker on NFS
docker info | grep "Docker Root Dir"

# k3d cluster
kubectl get nodes

# Ollama
curl localhost:11434/api/tags
ollama list | grep mistral
```

**Step 6: Commit final state**

```bash
git add .
git commit -m "feat: complete CollimaLab Pulumi deployment"
git push
```

---

## Validation Checklist

- [ ] NFS export exists on Unraid: `ssh root@10.0.0.10 "cat /etc/exports | grep colimalab"`
- [ ] NFS mounted on Mac: `mount | grep CollimaLab`
- [ ] Colima running: `colima status`
- [ ] Docker data on NFS: `docker info | grep "Docker Root Dir"` shows `/Volumes/CollimaLab/docker`
- [ ] k3d cluster ready: `kubectl get nodes` shows Ready
- [ ] Ollama responding: `curl localhost:11434/api/tags`
- [ ] Mistral model loaded: `ollama list | grep mistral-small`
