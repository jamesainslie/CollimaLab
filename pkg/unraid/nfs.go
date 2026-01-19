package unraid

import (
	"fmt"

	"github.com/pulumi/pulumi-command/sdk/go/command/local"
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

	// Build SSH command that will run during 'pulumi up'
	exportLine := fmt.Sprintf("%s %s(rw,async,no_subtree_check,no_root_squash,all_squash,anonuid=0,anongid=0)",
		args.ExportPath, args.Network)

	setupScript := fmt.Sprintf(`#!/bin/bash
set -e

SSH_TARGET="%s@%s"
EXPORT_PATH="%s"
EXPORT_LINE='%s'

echo "=== Testing SSH connectivity to Unraid ==="
ssh -o BatchMode=yes -o StrictHostKeyChecking=accept-new $SSH_TARGET "echo 'SSH connection successful'"

echo "=== Creating export directory ==="
ssh $SSH_TARGET "mkdir -p $EXPORT_PATH/docker && chmod 777 $EXPORT_PATH"

echo "=== Checking if NFS export exists ==="
if ssh $SSH_TARGET "grep -q '$EXPORT_PATH' /etc/exports 2>/dev/null"; then
    echo "NFS export already exists"
else
    echo "Adding NFS export..."
    ssh $SSH_TARGET "echo '$EXPORT_LINE' >> /etc/exports"
    echo "Reloading NFS exports..."
    ssh $SSH_TARGET "exportfs -ra"
fi

echo "=== NFS export setup complete ==="
`, args.User, args.Host, args.ExportPath, exportLine)

	// Use pulumi-command to defer execution until 'pulumi up'
	setup, err := local.NewCommand(ctx, name+"-setup", &local.CommandArgs{
		Create: pulumi.String(setupScript),
	}, pulumi.Parent(component))
	if err != nil {
		return nil, fmt.Errorf("failed to create NFS export command: %w", err)
	}

	component.ExportPath = setup.Stdout.ApplyT(func(_ string) string {
		return args.ExportPath
	}).(pulumi.StringOutput)

	if err := ctx.RegisterResourceOutputs(component, pulumi.Map{
		"exportPath": component.ExportPath,
	}); err != nil {
		return nil, err
	}

	return component, nil
}
