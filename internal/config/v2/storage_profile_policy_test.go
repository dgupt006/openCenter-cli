package v2

import (
	"strings"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
)

func TestNewV2DefaultUsesExplicitExternalStorageProfile(t *testing.T) {
	cfg, err := NewV2Default("storage-profile-default", "kind")
	if err != nil {
		t.Fatalf("NewV2Default: %v", err)
	}
	profile := cfg.OpenCenter.Infrastructure.Storage.Profile
	if profile.Lifecycle != StorageLifecycleNonProduction || profile.PVCProvider != StoragePVCProviderExternal || profile.ObjectStorageProvider != StorageObjectProviderExternalS3 {
		t.Fatalf("unexpected default storage profile: %#v", profile)
	}
	if backend := ResolveObjectStorageBackend(cfg, "loki"); backend != "s3" {
		t.Fatalf("object storage backend = %q, want s3", backend)
	}
}

func TestStorageProfilePolicy(t *testing.T) {
	tests := []struct {
		name       string
		configure  func(*Config)
		issuePaths []string
	}{
		{
			name: "production requires external S3",
			configure: func(cfg *Config) {
				cfg.OpenCenter.Infrastructure.Storage.Profile = StorageProfileConfig{Lifecycle: StorageLifecycleProduction, PVCProvider: StoragePVCProviderLonghorn, ObjectStorageProvider: StorageObjectProviderRustFS}
				cfg.OpenCenter.Services["longhorn"].(*services.LonghornConfig).Enabled = true
			},
			issuePaths: []string{"opencenter.infrastructure.storage.profile.object_storage_provider"},
		},
		{
			name: "RustFS requires Longhorn provider and service",
			configure: func(cfg *Config) {
				cfg.OpenCenter.Infrastructure.Storage.Profile = StorageProfileConfig{Lifecycle: StorageLifecycleNonProduction, PVCProvider: StoragePVCProviderExternal, ObjectStorageProvider: StorageObjectProviderRustFS}
			},
			issuePaths: []string{"opencenter.infrastructure.storage.profile.pvc_provider", "opencenter.services.longhorn"},
		},
		{
			name: "external S3 requires an endpoint for every enabled consumer",
			configure: func(cfg *Config) {
				cfg.OpenCenter.Infrastructure.Storage.Profile = StorageProfileConfig{Lifecycle: StorageLifecycleProduction, PVCProvider: StoragePVCProviderExternal, ObjectStorageProvider: StorageObjectProviderExternalS3}
				velero := cfg.OpenCenter.Services["velero"].(*services.VeleroConfig)
				velero.Enabled = true
				velero.S3Endpoint = ""
			},
			issuePaths: []string{"opencenter.services.velero.s3_endpoint"},
		},
		{
			name: "external S3 Mimir requires an endpoint",
			configure: func(cfg *Config) {
				cfg.OpenCenter.Infrastructure.Storage.Profile = StorageProfileConfig{Lifecycle: StorageLifecycleProduction, PVCProvider: StoragePVCProviderExternal, ObjectStorageProvider: StorageObjectProviderExternalS3}
				cfg.OpenCenter.Services["mimir"].(*services.MimirConfig).Enabled = true
			},
			issuePaths: []string{"opencenter.services.mimir.s3_endpoint"},
		},
		{
			name: "Swift migration is explicit",
			configure: func(cfg *Config) {
				cfg.OpenCenter.Services["loki"].(*services.LokiConfig).StorageType = "swift"
			},
			issuePaths: []string{"opencenter.services.loki.storage_type"},
		},
		{
			name: "local filesystem bulk backend is rejected",
			configure: func(cfg *Config) {
				cfg.OpenCenter.Services["tempo"].(*services.TempoConfig).StorageType = "filesystem"
			},
			issuePaths: []string{"opencenter.services.tempo.storage_type"},
		},
		{
			name: "non-production RustFS needs no external object credentials",
			configure: func(cfg *Config) {
				cfg.OpenCenter.Infrastructure.Storage.Profile = StorageProfileConfig{Lifecycle: StorageLifecycleNonProduction, PVCProvider: StoragePVCProviderLonghorn, ObjectStorageProvider: StorageObjectProviderRustFS}
				cfg.OpenCenter.Services["longhorn"].(*services.LonghornConfig).Enabled = true
				cfg.OpenCenter.Services["harbor"].(*services.HarborConfig).Enabled = true
				cfg.OpenCenter.Services["mimir"].(*services.MimirConfig).Enabled = true
				cfg.OpenCenter.Services["etcd-backup"].(*services.EtcdBackupConfig).Enabled = true
				cfg.Secrets.Loki.S3AccessKeyID = ""
				cfg.Secrets.Loki.S3SecretAccessKey = ""
				cfg.Secrets.Tempo.AccessKey = ""
				cfg.Secrets.Tempo.SecretKey = ""
				cfg.Secrets.Harbor.S3AccessKeyID = ""
				cfg.Secrets.Harbor.S3SecretAccessKey = ""
				cfg.Secrets.Mimir.SwiftApplicationCredentialSecret = ""
				cfg.Secrets.EtcdBackup.AccessKeyID = ""
				cfg.Secrets.EtcdBackup.SecretAccessKey = ""
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validReadinessConfig(t, "kind")
			tt.configure(cfg)
			report := ValidateReadiness(cfg)
			for _, path := range tt.issuePaths {
				assertIssue(t, report, SeverityError, CategoryServices, path)
			}
			if len(tt.issuePaths) == 0 {
				for _, issue := range report.Issues {
					if strings.Contains(issue.Path, "s3_") || strings.Contains(issue.Path, "swift_") {
						t.Fatalf("managed RustFS profile must not require external object-storage field %q: %s", issue.Path, issue.Message)
					}
				}
			}
		})
	}
}

func TestStorageProfilePolicyAppliesToDeploymentValidation(t *testing.T) {
	cfg := validReadinessConfig(t, "kind")
	cfg.OpenCenter.Infrastructure.Storage.Profile = StorageProfileConfig{Lifecycle: StorageLifecycleProduction, PVCProvider: StoragePVCProviderLonghorn, ObjectStorageProvider: StorageObjectProviderRustFS}
	cfg.OpenCenter.Services["longhorn"].(*services.LonghornConfig).Enabled = true

	err := ValidateForDeployment(cfg)
	if err == nil || !strings.Contains(err.Error(), "RustFS is non-production only") {
		t.Fatalf("expected production RustFS policy error, got %v", err)
	}
}

func TestNewV2DefaultGeneratesRustFSCredentials(t *testing.T) {
	cfg, err := NewV2Default("rustfs-credentials", "kind")
	if err != nil {
		t.Fatalf("NewV2Default: %v", err)
	}
	if isMissingSecret(cfg.Secrets.RustFS.AccessKey) || isMissingSecret(cfg.Secrets.RustFS.SecretKey) {
		t.Fatalf("NewV2Default must generate RustFS credentials: %#v", cfg.Secrets.RustFS)
	}
}

func TestStorageProfilePermittedReleaseMatrix(t *testing.T) {
	tests := []struct {
		name     string
		profile  StorageProfileConfig
		longhorn bool
	}{
		{name: "production external S3 external CSI", profile: StorageProfileConfig{Lifecycle: StorageLifecycleProduction, PVCProvider: StoragePVCProviderExternal, ObjectStorageProvider: StorageObjectProviderExternalS3}},
		{name: "production external S3 Longhorn PVCs", profile: StorageProfileConfig{Lifecycle: StorageLifecycleProduction, PVCProvider: StoragePVCProviderLonghorn, ObjectStorageProvider: StorageObjectProviderExternalS3}, longhorn: true},
		// Edge Production is governed by the production lifecycle policy. It must
		// therefore use external S3 even when its PVCs come from an external CSI.
		{name: "edge production external S3", profile: StorageProfileConfig{Lifecycle: StorageLifecycleProduction, PVCProvider: StoragePVCProviderExternal, ObjectStorageProvider: StorageObjectProviderExternalS3}},
		{name: "non-production Longhorn RustFS", profile: StorageProfileConfig{Lifecycle: StorageLifecycleNonProduction, PVCProvider: StoragePVCProviderLonghorn, ObjectStorageProvider: StorageObjectProviderRustFS}, longhorn: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validReadinessConfig(t, "kind")
			cfg.OpenCenter.Infrastructure.Storage.Profile = tt.profile
			cfg.OpenCenter.Services["longhorn"].(*services.LonghornConfig).Enabled = tt.longhorn
			if issues := storagePolicyIssues(cfg); len(issues) != 0 {
				t.Fatalf("permitted profile produced policy issues: %#v", issues)
			}
		})
	}
}

func TestStorageProfileAllowsNoObjectStorageConsumers(t *testing.T) {
	cfg := validReadinessConfig(t, "kind")
	cfg.OpenCenter.Infrastructure.Storage.Profile = StorageProfileConfig{
		Lifecycle: StorageLifecycleProduction, PVCProvider: StoragePVCProviderExternal, ObjectStorageProvider: StorageObjectProviderExternalS3,
	}
	cfg.OpenCenter.Services["loki"].(*services.LokiConfig).Enabled = false
	cfg.OpenCenter.Services["tempo"].(*services.TempoConfig).Enabled = false
	cfg.OpenCenter.Services["mimir"].(*services.MimirConfig).Enabled = false
	cfg.OpenCenter.Services["velero"].(*services.VeleroConfig).Enabled = false
	cfg.OpenCenter.Services["harbor"].(*services.HarborConfig).Enabled = false
	cfg.OpenCenter.Services["etcd-backup"].(*services.EtcdBackupConfig).Enabled = false
	if issues := storagePolicyIssues(cfg); len(issues) != 0 {
		t.Fatalf("profile with no object-storage consumers produced policy issues: %#v", issues)
	}
}
