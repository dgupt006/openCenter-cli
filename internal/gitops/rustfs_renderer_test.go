package gitops

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/secretartifacts"
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

func TestRustFSRendersGeneratedCredentialsBucketsAndConsumerS3Values(t *testing.T) {
	cfg := rustFSConfig(t)
	cfg.OpenCenter.Services["velero"].(*services.VeleroConfig).Enabled = true
	cfg.OpenCenter.Services["mimir"].(*services.MimirConfig).Enabled = true
	cfg.OpenCenter.Services["harbor"].(*services.HarborConfig).Enabled = true
	cfg.OpenCenter.Services["etcd-backup"].(*services.EtcdBackupConfig).Enabled = true
	cfg.OpenCenter.GitOps.Repository.LocalDir = t.TempDir()
	require.NoError(t, RenderClusterApps(cfg))

	overlay := filepath.Join(cfg.OpenCenter.GitOps.Repository.LocalDir, "applications", "overlays", cfg.ClusterName(), "services")
	credentials := mustReadFile(t, filepath.Join(overlay, "rustfs", "secret.yaml"))
	require.Contains(t, credentials, cfg.Secrets.RustFS.AccessKey)
	require.Contains(t, credentials, cfg.Secrets.RustFS.SecretKey)

	bootstrap := mustReadFile(t, filepath.Join(overlay, "rustfs", "bucket-bootstrap-job.yaml"))
	for _, serviceName := range []string{"loki", "tempo", "mimir", "velero", "harbor", "etcd-backup"} {
		require.Contains(t, bootstrap, cfg.ManagedObjectStorageBucket(serviceName))
	}

	for _, serviceName := range []string{"loki", "tempo", "mimir", "velero", "harbor"} {
		values := mustReadFile(t, filepath.Join(overlay, serviceName, "helm-values", "override-values.yaml"))
		endpoint := cfg.ManagedObjectStorageEndpoint()
		if serviceName == "tempo" {
			endpoint = strings.TrimPrefix(strings.TrimPrefix(endpoint, "https://"), "http://")
		}
		require.Contains(t, values, endpoint, "%s must use the internal RustFS endpoint", serviceName)
		require.NotContains(t, values, "backend: swift", "%s must not retain Swift storage rendering", serviceName)
	}

	artifacts, err := secretartifacts.Plan(&cfg)
	require.NoError(t, err)
	var etcdArtifact secretartifacts.Artifact
	for _, artifact := range artifacts {
		if artifact.TargetService == "etcd-backup" {
			etcdArtifact = artifact
			break
		}
	}
	require.Equal(t, cfg.ManagedObjectStorageEndpoint(), etcdArtifact.Payload["S3_HOST"])
	require.Equal(t, cfg.ManagedObjectStorageBucket("etcd-backup"), etcdArtifact.Payload["S3_BUCKET_NAME"])

	for _, serviceName := range []string{"loki", "tempo", "mimir", "velero", "harbor", "etcd-backup"} {
		flux := mustReadFile(t, filepath.Join(overlay, "fluxcd", serviceName+".yaml"))
		require.Contains(t, flux, "name: rustfs", "%s must wait for RustFS buckets and credentials", serviceName)
	}
}
