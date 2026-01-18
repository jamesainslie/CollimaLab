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
