package gitops

import (
	"path/filepath"
	"testing"

	configservices "github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	"gopkg.in/yaml.v3"
)

type keycloakCustomResource struct {
	Spec struct {
		StartOptimized bool `yaml:"startOptimized"`
		Instances      int  `yaml:"instances"`
		Resources      struct {
			Requests struct {
				CPU string `yaml:"cpu"`
			} `yaml:"requests"`
			Limits struct {
				CPU string `yaml:"cpu"`
			} `yaml:"limits"`
		} `yaml:"resources"`
	} `yaml:"spec"`
}

func TestOCTR654KeycloakRenderedDefaults(t *testing.T) {
	cfg := newDefault("octr654-defaults")
	dst := t.TempDir()
	cfg.OpenCenter.GitOps.Repository.LocalDir = dst

	if err := RenderClusterApps(cfg); err != nil {
		t.Fatalf("RenderClusterApps() error = %v", err)
	}

	cr := readRenderedKeycloakCR(t, dst, cfg.ClusterName())
	if cr.Spec.StartOptimized {
		t.Fatal("default Keycloak CR startOptimized = true, want false")
	}
	if got := cr.Spec.Resources.Requests.CPU; got != "500m" {
		t.Fatalf("default Keycloak CR request CPU = %q, want 500m", got)
	}
	if got := cr.Spec.Resources.Limits.CPU; got != "2" {
		t.Fatalf("default Keycloak CR limit CPU = %q, want 2", got)
	}
	if got := cr.Spec.Instances; got != 3 {
		t.Fatalf("default Keycloak CR instances = %d, want 3", got)
	}
}

func TestOCTR654KeycloakRenderedOverrides(t *testing.T) {
	cfg := newDefault("octr654-overrides")
	dst := t.TempDir()
	cfg.OpenCenter.GitOps.Repository.LocalDir = dst

	keycloak := cfg.OpenCenter.Services["keycloak"].(*configservices.KeycloakConfig)
	keycloak.StartOptimized = true
	keycloak.ResourceRequestsCPU = "750m"
	keycloak.ResourceLimitsCPU = "4"
	keycloak.Instances = 5

	if err := RenderClusterApps(cfg); err != nil {
		t.Fatalf("RenderClusterApps() error = %v", err)
	}

	cr := readRenderedKeycloakCR(t, dst, cfg.ClusterName())
	if !cr.Spec.StartOptimized {
		t.Fatal("explicit Keycloak CR startOptimized = false, want true")
	}
	if got := cr.Spec.Resources.Requests.CPU; got != "750m" {
		t.Fatalf("explicit Keycloak CR request CPU = %q, want 750m", got)
	}
	if got := cr.Spec.Resources.Limits.CPU; got != "4" {
		t.Fatalf("explicit Keycloak CR limit CPU = %q, want 4", got)
	}
	if got := cr.Spec.Instances; got != 5 {
		t.Fatalf("explicit Keycloak CR instances = %d, want 5", got)
	}
}

func readRenderedKeycloakCR(t *testing.T, root, clusterName string) keycloakCustomResource {
	t.Helper()

	path := filepath.Join(root, "applications", "overlays", clusterName, "services", "keycloak", "20-keycloak", "keycloak-cr-patch.yaml")
	var cr keycloakCustomResource
	if err := yaml.Unmarshal([]byte(mustReadFile(t, path)), &cr); err != nil {
		t.Fatalf("parse rendered Keycloak CR: %v", err)
	}
	return cr
}
