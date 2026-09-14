package v2

import (
	"strings"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
)

func TestNewDefaultServiceConfigReturnsPublicDefaultsWithoutRendererMetadata(t *testing.T) {
	service, ok := NewDefaultServiceConfig("keycloak", "example.com")
	if !ok {
		t.Fatal("NewDefaultServiceConfig() did not recognize keycloak")
	}

	keycloak, ok := service.(*services.KeycloakConfig)
	if !ok {
		t.Fatalf("service type = %T, want *services.KeycloakConfig", service)
	}
	if !keycloak.Enabled || keycloak.Namespace != "keycloak" {
		t.Fatalf("public defaults = %#v, want enabled keycloak service in keycloak namespace", keycloak.BaseConfig)
	}
	if keycloak.Hostname != "auth.example.com" {
		t.Fatalf("hostname = %q, want auth.example.com", keycloak.Hostname)
	}
	if keycloak.Source != (services.ServiceSource{}) || keycloak.Image != (services.ServiceImage{}) {
		t.Fatalf("factory persisted renderer-owned metadata: source=%#v image=%#v", keycloak.Source, keycloak.Image)
	}
}

func TestDefaultStorageClassSeparatesKubernetesClassFromBlockVolumeType(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		region   string
		want     string
	}{
		{name: "openstack", provider: "openstack", region: "DFW3", want: "csi-cinder-sc-delete"},
		{name: "kind", provider: "kind", region: "", want: "standard"},
		{name: "bare metal", provider: "baremetal", region: "", want: "standard"},
		{name: "vmware", provider: "vmware", region: "", want: "vsphere-csi"},
		{name: "fallback", provider: "unknown", region: "", want: "standard"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := defaultStorageClass(tt.provider, tt.region); got != tt.want {
				t.Fatalf("defaultStorageClass(%q, %q) = %q, want %q", tt.provider, tt.region, got, tt.want)
			}
		})
	}

	cfg, err := NewV2Default("storage-types", "openstack")
	if err != nil {
		t.Fatalf("NewV2Default() error = %v", err)
	}
	if got := cfg.OpenCenter.Infrastructure.Storage.WorkerVolumeType; got != "HA-Standard" {
		t.Fatalf("OpenStack worker volume type = %q, want HA-Standard", got)
	}
	if got := cfg.OpenCenter.Infrastructure.Storage.DefaultStorageClass; got != "csi-cinder-sc-delete" {
		t.Fatalf("OpenStack default storage class = %q, want csi-cinder-sc-delete", got)
	}
}

func TestValidateForDeploymentMatchesReadinessForSwiftMigrationError(t *testing.T) {
	cfg := readinessTestConfigForDeployment(t)
	cfg.OpenCenter.Services["tempo"].(*services.TempoConfig).StorageType = "swift"

	readiness := ValidateReadiness(cfg)
	assertIssue(t, readiness, SeverityError, CategoryServices, "opencenter.services.tempo.storage_type")

	err := ValidateForDeployment(cfg)
	if err == nil || !strings.Contains(err.Error(), "opencenter.services.tempo.storage_type") || !strings.Contains(err.Error(), "Swift is no longer supported") {
		t.Fatalf("expected matching Swift migration error from deployment validation, got %v", err)
	}
}

func readinessTestConfigForDeployment(t *testing.T) *Config {
	t.Helper()
	cfg := validReadinessConfig(t, "kind")
	cfg.Secrets.Loki.SwiftApplicationCredentialSecret = "loki-swift-secret"
	cfg.Secrets.Loki.S3AccessKeyID = "loki-s3-access"
	cfg.Secrets.Loki.S3SecretAccessKey = "loki-s3-secret"
	cfg.Secrets.Tempo.SwiftApplicationCredentialSecret = "tempo-swift-secret"
	cfg.Secrets.Tempo.AccessKey = "tempo-s3-access"
	cfg.Secrets.Tempo.SecretKey = "tempo-s3-secret"
	return cfg
}

func readinessHasPath(report ReadinessReport, path string) bool {
	for _, issue := range report.Issues {
		if issue.Path == path {
			return true
		}
	}
	return false
}

func TestResolveObjectStorageBackendAlwaysUsesPortableS3Contract(t *testing.T) {
	for _, provider := range []string{"openstack", "kind"} {
		for _, serviceName := range []string{"loki", "tempo"} {
			t.Run(provider+" "+serviceName, func(t *testing.T) {
				cfg := validReadinessConfig(t, provider)
				if got := ResolveObjectStorageBackend(cfg, serviceName); got != "s3" {
					t.Fatalf("ResolveObjectStorageBackend() = %q, want s3", got)
				}
			})
		}
	}
}

func TestOmittedGenericObjectStorageBackendRequiresS3Credentials(t *testing.T) {
	cfg := readinessTestConfigForDeployment(t)
	cfg.OpenCenter.Services["loki"].(*services.LokiConfig).StorageType = ""
	cfg.OpenCenter.Services["tempo"].(*services.TempoConfig).StorageType = ""
	cfg.Secrets.Loki.S3AccessKeyID = PlaceholderSecret
	cfg.Secrets.Loki.S3SecretAccessKey = PlaceholderSecret
	cfg.Secrets.Tempo.AccessKey = PlaceholderSecret
	cfg.Secrets.Tempo.SecretKey = PlaceholderSecret
	cfg.Secrets.Loki.SwiftApplicationCredentialSecret = "unused-loki-swift"
	cfg.Secrets.Tempo.SwiftApplicationCredentialSecret = "unused-tempo-swift"

	readiness := ValidateReadiness(cfg)
	deploymentText := ValidateForDeployment(cfg).Error()
	for _, path := range []string{
		"secrets.loki.s3_access_key_id",
		"secrets.loki.s3_secret_access_key",
		"secrets.tempo.access_key",
		"secrets.tempo.secret_key",
	} {
		if !readinessHasPath(readiness, path) || !strings.Contains(deploymentText, path) {
			t.Errorf("omitted generic backend did not require %s in both validators", path)
		}
	}
	for _, path := range []string{
		"secrets.loki.swift_application_credential_secret",
		"secrets.tempo.swift_application_credential_secret",
	} {
		if readinessHasPath(readiness, path) || strings.Contains(deploymentText, path) {
			t.Errorf("omitted generic backend incorrectly required %s", path)
		}
	}
}
