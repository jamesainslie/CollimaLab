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
