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
