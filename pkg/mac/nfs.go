package mac

import (
	"fmt"

	"github.com/pulumi/pulumi-command/sdk/go/command/local"
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

	nfsPath := fmt.Sprintf("%s:%s", args.ServerHost, args.ServerPath)

	setupScript := fmt.Sprintf(`#!/bin/bash
set -e

MOUNT_POINT="%s"
NFS_PATH="%s"

echo "=== Creating mount point directory ==="
mkdir -p "$MOUNT_POINT"

echo "=== Checking if already mounted ==="
if mount | grep -q "$MOUNT_POINT"; then
    echo "NFS share already mounted"
else
    echo "Mounting NFS share..."
    mount -t nfs -o resvport,rw,noatime "$NFS_PATH" "$MOUNT_POINT"
fi

echo "=== Verifying mount ==="
mount | grep "$MOUNT_POINT"

echo "=== NFS mount complete ==="
`, args.MountPoint, nfsPath)

	// Use pulumi-command to defer execution until 'pulumi up'
	setup, err := local.NewCommand(ctx, name+"-setup", &local.CommandArgs{
		Create: pulumi.String(setupScript),
		Delete: pulumi.String(fmt.Sprintf("umount %s 2>/dev/null || true", args.MountPoint)),
	}, pulumi.Parent(component))
	if err != nil {
		return nil, fmt.Errorf("failed to create NFS mount command: %w", err)
	}

	component.MountPoint = setup.Stdout.ApplyT(func(_ string) string {
		return args.MountPoint
	}).(pulumi.StringOutput)

	if err := ctx.RegisterResourceOutputs(component, pulumi.Map{
		"mountPoint": component.MountPoint,
	}); err != nil {
		return nil, err
	}

	return component, nil
}
