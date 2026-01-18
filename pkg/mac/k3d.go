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
