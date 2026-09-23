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

func TestValidateForDeploymentMatchesReadinessForSelectedLokiTempoBackends(t *testing.T) {
	tests := []struct {
		name          string
		configure     func(*Config)
		wantPaths     []string
		dontWantPaths []string
	}{
		{
			name: "loki s3 and tempo swift",
			configure: func(cfg *Config) {
				cfg.OpenCenter.Services["loki"].(*services.LokiConfig).StorageType = "s3"
				cfg.OpenCenter.Services["tempo"].(*services.TempoConfig).StorageType = "swift"
				cfg.Secrets.Loki.S3AccessKeyID = PlaceholderSecret
				cfg.Secrets.Loki.S3SecretAccessKey = PlaceholderSecret
				cfg.Secrets.Loki.SwiftApplicationCredentialSecret = "loki-swift-secret"
				cfg.Secrets.Tempo.SwiftApplicationCredentialSecret = PlaceholderSecret
				cfg.Secrets.Tempo.AccessKey = "tempo-s3-access"
				cfg.Secrets.Tempo.SecretKey = "tempo-s3-secret"
			},
			wantPaths: []string{
				"secrets.loki.s3_access_key_id",
				"secrets.loki.s3_secret_access_key",
				"secrets.tempo.swift_application_credential_secret",
			},
			dontWantPaths: []string{
				"secrets.loki.swift_application_credential_secret",
				"secrets.tempo.access_key",
				"secrets.tempo.secret_key",
			},
		},
		{
			name: "global application credentials satisfy selected s3 backends",
			configure: func(cfg *Config) {
				cfg.OpenCenter.Services["loki"].(*services.LokiConfig).StorageType = "s3"
				cfg.OpenCenter.Services["tempo"].(*services.TempoConfig).StorageType = "s3"
				cfg.Secrets.Global.AWS.Application.AccessKey = "global-access"
				cfg.Secrets.Global.AWS.Application.SecretAccessKey = "global-secret"
				cfg.Secrets.Loki.S3AccessKeyID = ""
				cfg.Secrets.Loki.S3SecretAccessKey = ""
				cfg.Secrets.Tempo.AccessKey = ""
				cfg.Secrets.Tempo.SecretKey = ""
				cfg.Secrets.Loki.SwiftApplicationCredentialSecret = PlaceholderSecret
				cfg.Secrets.Tempo.SwiftApplicationCredentialSecret = PlaceholderSecret
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := readinessTestConfigForDeployment(t)
			tt.configure(cfg)

			readiness := ValidateReadiness(cfg)
			deploymentErr := ValidateForDeployment(cfg)
			deploymentText := ""
			if deploymentErr != nil {
				deploymentText = deploymentErr.Error()
			}

			for _, path := range tt.wantPaths {
				if !readinessHasPath(readiness, path) || !strings.Contains(deploymentText, path) {
					t.Errorf("selected backend path %q not reported by both validators; readiness=%v deployment=%q", path, readinessHasPath(readiness, path), deploymentText)
				}
			}
			for _, path := range tt.dontWantPaths {
				if readinessHasPath(readiness, path) || strings.Contains(deploymentText, path) {
					t.Errorf("unused backend path %q reported; readiness=%v deployment=%q", path, readinessHasPath(readiness, path), deploymentText)
				}
			}
		})
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

func TestResolveObjectStorageBackendProviderDefaultsAndExplicitOverrides(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		service  string
		explicit string
		want     string
	}{
		{name: "openstack loki omitted", provider: "openstack", service: "loki", want: "swift"},
		{name: "openstack tempo omitted", provider: "openstack", service: "tempo", want: "swift"},
		{name: "generic loki omitted", provider: "kind", service: "loki", want: "s3"},
		{name: "generic tempo omitted", provider: "kind", service: "tempo", want: "s3"},
		{name: "openstack explicit s3", provider: "openstack", service: "loki", explicit: "S3", want: "s3"},
		{name: "generic explicit swift", provider: "kind", service: "tempo", explicit: "SWIFT", want: "swift"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validReadinessConfig(t, tt.provider)
			switch tt.service {
			case "loki":
				cfg.OpenCenter.Services["loki"].(*services.LokiConfig).StorageType = tt.explicit
			case "tempo":
				cfg.OpenCenter.Services["tempo"].(*services.TempoConfig).StorageType = tt.explicit
			}
			if got := ResolveObjectStorageBackend(cfg, tt.service); got != tt.want {
				t.Fatalf("ResolveObjectStorageBackend() = %q, want %q", got, tt.want)
			}
		})
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
