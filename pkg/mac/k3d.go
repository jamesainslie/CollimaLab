package mac

import (
	"fmt"
	"strings"

	"github.com/pulumi/pulumi-command/sdk/go/command/local"
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
		return nil, fmt.Errorf("registering k3d cluster component: %w", err)
	}

	// Build port arguments
	portArgs := ""
	for _, port := range args.Ports {
		portArgs += fmt.Sprintf(" --port %s", port)
	}

	setupScript := fmt.Sprintf(`#!/bin/bash
set -e

CLUSTER_NAME="%s"
SERVERS=%d

echo "=== Installing k3d if not present ==="
if ! command -v k3d &> /dev/null; then
    brew install k3d
fi

echo "=== Installing kubectl if not present ==="
if ! command -v kubectl &> /dev/null; then
    brew install kubectl
fi

echo "=== Checking if cluster exists ==="
if k3d cluster list -o json 2>/dev/null | grep -q "\"name\":\"$CLUSTER_NAME\""; then
    echo "Cluster $CLUSTER_NAME already exists"
else
    echo "Creating k3d cluster $CLUSTER_NAME..."
    k3d cluster create "$CLUSTER_NAME" --servers $SERVERS%s
fi

echo "=== Waiting for cluster to be ready ==="
sleep 10

echo "=== Verifying cluster ==="
kubectl get nodes

echo "=== k3d cluster setup complete ==="
`, args.Name, args.Servers, portArgs)

	// Use pulumi-command to defer execution until 'pulumi up'
	setup, err := local.NewCommand(ctx, name+"-setup", &local.CommandArgs{
		Create: pulumi.String(setupScript),
		Delete: pulumi.String(fmt.Sprintf("k3d cluster delete %s 2>/dev/null || true", args.Name)),
	}, pulumi.Parent(component))
	if err != nil {
		return nil, fmt.Errorf("failed to create k3d cluster command: %w", err)
	}

	component.Name = pulumi.String(args.Name).ToStringOutput()
	component.Status = setup.Stdout.ApplyT(func(stdout string) string {
		if strings.Contains(stdout, "Ready") {
			return "running"
		}
		return "created"
	}).(pulumi.StringOutput)

	if err := ctx.RegisterResourceOutputs(component, pulumi.Map{
		"name":   component.Name,
		"status": component.Status,
	}); err != nil {
		return nil, err
	}

	return component, nil
}
