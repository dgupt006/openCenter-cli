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

// TestStorageProfileReleaseMatrixGeneratesConsumerTrees is an offline,
// air-gap-safe release gate: it performs no provider, registry, Kubernetes, or
// Git calls while proving every bulk-data consumer renders for each permitted
// storage profile. A live Kind smoke remains an explicit operator exercise.
func TestStorageProfileReleaseMatrixGeneratesConsumerTrees(t *testing.T) {
	cases := []struct {
		name    string
		profile v2.StorageProfileConfig
		managed bool
	}{
		{
			name:    "production external S3 with external CSI",
			profile: v2.StorageProfileConfig{Lifecycle: v2.StorageLifecycleProduction, PVCProvider: v2.StoragePVCProviderExternal, ObjectStorageProvider: v2.StorageObjectProviderExternalS3},
		},
		{
			name:    "production external S3 with Longhorn PVCs",
			profile: v2.StorageProfileConfig{Lifecycle: v2.StorageLifecycleProduction, PVCProvider: v2.StoragePVCProviderLonghorn, ObjectStorageProvider: v2.StorageObjectProviderExternalS3},
		},
		{
			name:    "edge production follows the production external S3 policy",
			profile: v2.StorageProfileConfig{Lifecycle: v2.StorageLifecycleProduction, PVCProvider: v2.StoragePVCProviderExternal, ObjectStorageProvider: v2.StorageObjectProviderExternalS3},
		},
		{
			name:    "non-production Longhorn RustFS",
			profile: v2.StorageProfileConfig{Lifecycle: v2.StorageLifecycleNonProduction, PVCProvider: v2.StoragePVCProviderLonghorn, ObjectStorageProvider: v2.StorageObjectProviderRustFS},
			managed: true,
		},
	}

	consumers := []string{"loki", "tempo", "mimir", "velero", "harbor", "etcd-backup"}
	valueConsumers := []string{"loki", "tempo", "mimir", "velero", "harbor"}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := v2.NewV2Default("storage-release", "kind")
			require.NoError(t, err)
			cfg.OpenCenter.Infrastructure.Storage.Profile = tt.profile
			if tt.profile.PVCProvider == v2.StoragePVCProviderLonghorn {
				cfg.OpenCenter.Services["longhorn"].(*services.LonghornConfig).Enabled = true
			}
			for _, service := range consumers {
				enableObjectStorageConsumer(t, cfg, service, tt.managed)
			}
			cfg.OpenCenter.GitOps.Repository.LocalDir = t.TempDir()
			require.NoError(t, RenderClusterApps(*cfg))

			overlay := filepath.Join(cfg.OpenCenter.GitOps.Repository.LocalDir, "applications", "overlays", cfg.ClusterName(), "services")
			for _, service := range consumers {
				flux := mustReadFile(t, filepath.Join(overlay, "fluxcd", service+".yaml"))
				if tt.managed {
					require.Contains(t, flux, "name: rustfs", "%s must wait for managed RustFS", service)
				} else {
					require.NotContains(t, flux, "name: rustfs", "%s must not depend on RustFS for external S3", service)
				}
			}
			for _, service := range valueConsumers {
				values := mustReadFile(t, filepath.Join(overlay, service, "helm-values", "override-values.yaml"))
				require.NotContains(t, values, "backend: swift", "%s must never render Swift", service)
				if tt.managed {
					endpoint := cfg.ManagedObjectStorageEndpoint()
					if service == "tempo" {
						endpoint = strings.TrimPrefix(strings.TrimPrefix(endpoint, "https://"), "http://")
					}
					require.Contains(t, values, endpoint, "%s must use the internal RustFS endpoint", service)
				} else {
					endpoint := "https://s3.release.example"
					if service == "tempo" {
						endpoint = "s3.release.example"
					}
					require.Contains(t, values, endpoint, "%s must use the external S3 endpoint", service)
				}
			}

			artifacts, err := secretartifacts.Plan(cfg)
			require.NoError(t, err)
			etcd := findArtifact(t, artifacts, "etcd-backup")
			if tt.managed {
				require.Equal(t, cfg.ManagedObjectStorageEndpoint(), etcd.Payload["S3_HOST"])
			} else {
				require.Equal(t, "https://s3.release.example", etcd.Payload["S3_HOST"])
				require.NotContains(t, mustReadFile(t, filepath.Join(overlay, "fluxcd", "kustomization.yaml")), "./rustfs.yaml")
			}
		})
	}
}

func enableObjectStorageConsumer(t *testing.T, cfg *v2.Config, service string, managed bool) {
	t.Helper()
	endpoint := "https://s3.release.example"
	access, secret := "release-access", "release-secret"
	if managed {
		// Rendering must source these values from generated RustFS credentials,
		// not from external S3 fields supplied by the caller.
		endpoint, access, secret = "", "", ""
	}
	switch service {
	case "loki":
		item := cfg.OpenCenter.Services[service].(*services.LokiConfig)
		item.Enabled, item.S3Endpoint, item.BucketName = true, endpoint, "release-loki"
		cfg.Secrets.Loki.S3AccessKeyID, cfg.Secrets.Loki.S3SecretAccessKey = access, secret
	case "tempo":
		item := cfg.OpenCenter.Services[service].(*services.TempoConfig)
		item.Enabled, item.S3Endpoint, item.BucketName = true, endpoint, "release-tempo"
		cfg.Secrets.Tempo.AccessKey, cfg.Secrets.Tempo.SecretKey = access, secret
	case "mimir":
		item := cfg.OpenCenter.Services[service].(*services.MimirConfig)
		item.Enabled, item.S3Endpoint, item.S3BucketName = true, endpoint, "release-mimir"
		cfg.Secrets.Mimir.S3AccessKeyID, cfg.Secrets.Mimir.S3SecretAccessKey = access, secret
	case "velero":
		item := cfg.OpenCenter.Services[service].(*services.VeleroConfig)
		item.Enabled, item.S3Endpoint, item.BackupBucket = true, endpoint, "release-velero"
		cfg.Secrets.Velero.AccessKeyID, cfg.Secrets.Velero.SecretAccessKey = access, secret
	case "harbor":
		item := cfg.OpenCenter.Services[service].(*services.HarborConfig)
		item.Enabled, item.S3Endpoint, item.S3Bucket = true, endpoint, "release-harbor"
		cfg.Secrets.Harbor.S3AccessKeyID, cfg.Secrets.Harbor.S3SecretAccessKey = access, secret
	case "etcd-backup":
		item := cfg.OpenCenter.Services[service].(*services.EtcdBackupConfig)
		item.Enabled, item.S3Endpoint, item.S3BucketName = true, endpoint, "release-etcd-backup"
		cfg.Secrets.EtcdBackup.AccessKeyID, cfg.Secrets.EtcdBackup.SecretAccessKey = access, secret
	default:
		t.Fatalf("unknown object-storage consumer %q", service)
	}
}

func findArtifact(t *testing.T, artifacts []secretartifacts.Artifact, service string) secretartifacts.Artifact {
	t.Helper()
	for _, artifact := range artifacts {
		if artifact.TargetService == service {
			return artifact
		}
	}
	t.Fatalf("missing %s secret artifact", service)
	return secretartifacts.Artifact{}
}
