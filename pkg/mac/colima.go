package mac

import (
	"fmt"
	"os"

	"github.com/pulumi/pulumi-command/sdk/go/command/local"
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

	// Build the setup script that will run during 'pulumi up'
	setupScript := fmt.Sprintf(`#!/bin/bash
set -e

echo "=== Stopping existing Colima if running ==="
colima stop 2>/dev/null || true

echo "=== Backing up existing config ==="
if [ -d "%s/.colima" ]; then
    rm -rf "%s/.colima.bak"
    mv "%s/.colima" "%s/.colima.bak"
fi

echo "=== Cleaning up Lima VM ==="
rm -rf "%s/.lima/colima"

echo "=== Uninstalling existing Colima and Docker ==="
brew uninstall colima 2>/dev/null || true
brew uninstall docker 2>/dev/null || true

echo "=== Installing fresh Colima and Docker CLI ==="
brew install colima
brew install docker

echo "=== Starting Colima with configuration ==="
colima start --cpu %d --memory %d --disk %d --vm-type vz --mount-type virtiofs --mount %s:w

echo "=== Configuring Docker data-root ==="
colima ssh -- 'sudo mkdir -p /etc/docker && echo '\''{"data-root": "%s"}'\'' | sudo tee /etc/docker/daemon.json && sudo systemctl restart docker'

echo "=== Waiting for Docker to be ready ==="
sleep 5

echo "=== Verifying Docker ==="
docker info

echo "=== Colima setup complete ==="
`, homeDir, homeDir, homeDir, homeDir, homeDir,
		args.CPU, args.Memory, args.Disk, args.NFSMount, args.DockerRoot)

	// Use pulumi-command to defer execution until 'pulumi up'
	setup, err := local.NewCommand(ctx, name+"-setup", &local.CommandArgs{
		Create: pulumi.String(setupScript),
		Delete: pulumi.String("colima stop 2>/dev/null || true"),
	}, pulumi.Parent(component))
	if err != nil {
		return nil, fmt.Errorf("failed to create colima setup command: %w", err)
	}

	component.Status = setup.Stdout.ApplyT(func(stdout string) string {
		return "running"
	}).(pulumi.StringOutput)

	if err := ctx.RegisterResourceOutputs(component, pulumi.Map{
		"status": component.Status,
	}); err != nil {
		return nil, err
	}

	return component, nil
}
