package gitops

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/stretchr/testify/require"
)

func TestRustFSDoesNotRenderForExternalS3Profile(t *testing.T) {
	cfg, err := v2.NewV2Default("rustfs-external", "kind")
	require.NoError(t, err)

	actions, err := planClusterAppActions(*cfg)
	require.NoError(t, err)
	for _, action := range actions {
		require.NotContains(t, action.Output, "services/rustfs/", "external S3 profiles must not render RustFS")
		require.NotEqual(t, "services/fluxcd/rustfs.yaml", action.Output)
	}
}

func TestRustFSRendersLonghornBackedFluxWorkload(t *testing.T) {
	cfg := rustFSConfig(t)
	cfg.OpenCenter.GitOps.Repository.LocalDir = t.TempDir()
	require.NoError(t, RenderClusterApps(cfg))

	overlay := filepath.Join(cfg.OpenCenter.GitOps.Repository.LocalDir, "applications", "overlays", cfg.ClusterName(), "services")
	kustomization := mustReadFile(t, filepath.Join(overlay, "rustfs", "kustomization.yaml"))
	require.Contains(t, kustomization, "statefulset.yaml")

	statefulSet := mustReadFile(t, filepath.Join(overlay, "rustfs", "statefulset.yaml"))
	require.Contains(t, statefulSet, "kind: StatefulSet")
	require.Contains(t, statefulSet, "storageClassName: longhorn")
	require.Contains(t, statefulSet, "volumeClaimTemplates:")
	require.Contains(t, statefulSet, "containerPort: 9000")
	require.Contains(t, statefulSet, "image: rustfs/rustfs:1.0.0-rc.6")
	require.NotContains(t, statefulSet, ":latest")

	flux := mustReadFile(t, filepath.Join(overlay, "fluxcd", "rustfs.yaml"))
	docs, err := decodeYAMLDocuments([]byte(flux))
	require.NoError(t, err)
	rustFS := findFluxKustomization(t, docs, "rustfs")
	require.True(t, hasFluxDependency(t, rustFS, "longhorn-base"), "RustFS must wait for Longhorn's healthy base release")

	aggregator := mustReadFile(t, filepath.Join(overlay, "fluxcd", "kustomization.yaml"))
	require.Contains(t, aggregator, "./rustfs.yaml", "the services aggregator must reconcile RustFS")
}

func TestRustFSRendererRejectsIncompleteManagedProfile(t *testing.T) {
	cfg := rustFSConfig(t)
	cfg.OpenCenter.Services["longhorn"].(*services.LonghornConfig).Enabled = false

	_, err := planRustFSActions(cfg, nil)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "Longhorn service"))
}

func rustFSConfig(t *testing.T) v2.Config {
	t.Helper()
	cfg, err := v2.NewV2Default("rustfs", "kind")
	require.NoError(t, err)
	cfg.OpenCenter.Infrastructure.Storage.Profile = v2.StorageProfileConfig{
		Lifecycle:             v2.StorageLifecycleNonProduction,
		PVCProvider:           v2.StoragePVCProviderLonghorn,
		ObjectStorageProvider: v2.StorageObjectProviderRustFS,
	}
	cfg.OpenCenter.Services["longhorn"].(*services.LonghornConfig).Enabled = true
	return *cfg
}
