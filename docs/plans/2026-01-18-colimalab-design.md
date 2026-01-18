# CollimaLab Design

**Date:** 2026-01-18
**Status:** Approved

## Overview

Configure a local container development environment on zeus.local (Mac mini M4) with storage offloaded to an Unraid NAS (10.0.0.10) via NFS. Includes k3d for Kubernetes development and Ollama with Mistral Small 3 for local AI.

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        zeus.local (Mac mini M4)                  │
│                        10 CPU / 16GB RAM                         │
│                                                                  │
│  ~/.colima/              # Colima config (local)                │
│  ~/.lima/colima/         # VM disk image (local, ~2-10GB)       │
│  ~/.ollama/              # Ollama models (local, ~15GB)         │
│                                                                  │
│  ┌─────────────┐    ┌─────────────────────────────────────────┐ │
│  │   Colima    │───▶│  Docker daemon (data-root on NFS)       │ │
│  │   (Lima VM) │    │                                         │ │
│  │  10 CPU     │    │  ┌───────────────────────────────────┐  │ │
│  │  12GB RAM   │    │  │  k3d cluster(s)                   │  │ │
│  └─────────────┘    │  │  └── k3s nodes (Docker containers)│  │ │
│                     │  │      └── pods, images, PVs        │  │ │
│                     │  └───────────────────────────────────┘  │ │
│                     └──────────────┬──────────────────────────┘ │
│                                    │                            │
│  ┌─────────────┐                   │                            │
│  │   Ollama    │ ◀── M4 GPU        │                            │
│  │   (native)  │     acceleration  │                            │
│  │   :11434    │                   │                            │
│  └─────────────┘                   │                            │
└────────────────────────────────────┼────────────────────────────┘
                                     │ NFS mount: /Volumes/CollimaLab
                                     ▼
┌─────────────────────────────────────────────────────────────────┐
│                    10.0.0.10 (Unraid)                            │
│                    Ryzen 9 5950X / 125GB RAM                     │
│                                                                  │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │  /mnt/store/colimalab  (NVMe SSD - 932GB)                   ││
│  │  ├── docker/           # All Docker data                    ││
│  │  │   ├── images/       # Docker + k3s images                ││
│  │  │   ├── containers/   # Running container data             ││
│  │  │   └── volumes/      # Docker & k3s persistent volumes    ││
│  └─────────────────────────────────────────────────────────────┘│
│  NFS export: /mnt/store/colimalab → 10.0.0.0/24                 │
└─────────────────────────────────────────────────────────────────┘
```

## Storage Summary

| Component | Location | Size |
|-----------|----------|------|
| Colima VM | Local | ~2-10GB |
| Ollama models | Local | ~15GB |
| Docker images/volumes | NFS | Variable |
| k3d/k3s data | NFS | Variable |

**Total local:** ~17-25GB
**Everything else:** NFS (932GB NVMe available)

## Component Details

### NFS Server (Unraid)

**Export path:** `/mnt/store/colimalab`

**Export options:**
```
/mnt/store/colimalab 10.0.0.0/24(rw,async,no_subtree_check,no_root_squash,all_squash,anonuid=0,anongid=0)
```

| Setting | Value | Reason |
|---------|-------|--------|
| Network | `10.0.0.0/24` | LAN only |
| Access | `rw,async,no_subtree_check` | Performance + compatibility |
| Root squash | `no_root_squash` | Docker daemon runs as root |
| Map users | `all_squash,anonuid=0,anongid=0` | Simplify permissions |

### Colima (Mac)

**Configuration (`~/.colima/default/colima.yaml`):**
```yaml
cpu: 10
memory: 12
disk: 10
vmType: vz           # Virtualization.framework (Apple Silicon)
mountType: virtiofs  # Fast mounts
mounts:
  - location: /Volumes/CollimaLab
    writable: true
docker:
  data-root: /Volumes/CollimaLab/docker
```

### k3d

**Default cluster creation:**
```bash
k3d cluster create lab \
  --servers 1 \
  --port "80:80@loadbalancer" \
  --port "443:443@loadbalancer"
```

All k3s data stored via Docker on NFS automatically.

### Ollama

**Installation:** Native via Homebrew (for M4 GPU acceleration)

**Model:** `mistral-small:24b-instruct-2501-q4_K_M`

**Access from k3d pods:** `http://host.k3d.internal:11434`

## Pulumi Automation (Go)

### Why Pulumi over Terraform/OpenTofu

- **Real programming language** - Full Go control flow, functions, error handling
- **Natural for local operations** - SSH commands, local exec, file management
- **State management** - Tracks what's deployed, enables updates/destroys
- **Provider ecosystem** - Docker, Kubernetes providers for k3d management

### Project Structure

```
CollimaLab/
├── Pulumi.yaml              # Project definition
├── Pulumi.dev.yaml          # Dev stack config (hosts, settings)
├── main.go                  # Entry point
├── pkg/
│   ├── unraid/
│   │   └── nfs.go           # NFS export configuration via SSH
│   ├── mac/
│   │   ├── nfs.go           # NFS mount on Mac
│   │   ├── colima.go        # Colima install & config
│   │   ├── k3d.go           # k3d cluster management
│   │   └── ollama.go        # Ollama install & model
│   └── util/
│       ├── ssh.go           # SSH helper functions
│       └── exec.go          # Local command execution
├── go.mod
├── go.sum
└── README.md
```

### Configuration (Pulumi.dev.yaml)

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

### Execution Flow

```go
func main() {
    pulumi.Run(func(ctx *pulumi.Context) error {
        // 1. Configure NFS export on Unraid (SSH)
        nfsExport := unraid.NewNFSExport(ctx, ...)

        // 2. Mount NFS on Mac (local exec)
        nfsMount := mac.NewNFSMount(ctx, ..., pulumi.DependsOn(nfsExport))

        // 3. Uninstall old Colima, install fresh
        colima := mac.NewColima(ctx, ..., pulumi.DependsOn(nfsMount))

        // 4. Create k3d cluster
        k3d := mac.NewK3dCluster(ctx, ..., pulumi.DependsOn(colima))

        // 5. Install Ollama and pull model
        ollama := mac.NewOllama(ctx, ..., pulumi.DependsOn(colima))

        return nil
    })
}
```

### Commands

```bash
# Preview changes
pulumi preview

# Deploy everything
pulumi up

# Destroy (cleanup)
pulumi destroy

# View current state
pulumi stack
```

## Validation Tests

| Test | Command |
|------|---------|
| NFS mount works | `mount \| grep CollimaLab` |
| Colima starts | `colima status` |
| Docker works | `docker run hello-world` |
| Docker on NFS | `docker info \| grep "Docker Root Dir"` |
| k3d cluster | `kubectl get nodes` |
| Ollama responds | `curl localhost:11434/api/tags` |
| Mistral model loaded | `ollama list \| grep mistral-small` |

## Pre-flight Checks

| Check | Action if fails |
|-------|-----------------|
| Unraid reachable via SSH | Fail early with clear message |
| `/mnt/store` mounted on Unraid | Fail - NVMe not available |
| NFS port (2049) not blocked | Warn and provide firewall fix |
| Existing Colima running | Stop it before uninstall |
| Homebrew installed on Mac | Install it if missing |

## Rollback

- Old `~/.colima` backed up to `~/.colima.bak` before removal
- NFS export can be removed from Unraid manually if needed
- Pulumi tracks state - `pulumi destroy` reverses deployment
- Individual components can be removed by commenting out and running `pulumi up`
