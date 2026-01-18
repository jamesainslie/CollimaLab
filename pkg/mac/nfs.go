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
