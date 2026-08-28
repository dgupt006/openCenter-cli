package gitops

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	configservices "github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/sops"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/crypto"
	"gopkg.in/yaml.v3"
)

func TestRenderSingleServiceHarborSecretsAreYAMLSafe(t *testing.T) {
	cfg := newDefault("harbor-render")
	cfg.OpenCenter.GitOps.Repository.LocalDir = t.TempDir()
	cfg.OpenCenter.Services["harbor"].(*configservices.HarborConfig).Enabled = true
	cfg.Secrets.Harbor = v2.HarborSecrets{
		AdminPassword:    "admin:# password\\nnext",
		RegistryPassword: "registry: # password",
		DatabasePassword: "database\"password",
	}

	if err := RenderSingleService(cfg, "harbor", false); err != nil {
		t.Fatalf("RenderSingleService() error = %v", err)
	}

	path := filepath.Join(cfg.OpenCenter.GitOps.Repository.LocalDir, "applications", "overlays", cfg.ClusterName(), "services", "harbor", "helm-values", "override-values.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read rendered Harbor values: %v", err)
	}
	var values map[string]any
	if err := yaml.Unmarshal(data, &values); err != nil {
		t.Fatalf("rendered Harbor values are not valid YAML: %v\n%s", err, data)
	}

	if got := values["harborAdminPassword"]; got != cfg.Secrets.Harbor.AdminPassword {
		t.Fatalf("harborAdminPassword = %#v, want %q", got, cfg.Secrets.Harbor.AdminPassword)
	}
	registry := values["registry"].(map[string]any)["credentials"].(map[string]any)
	if got := registry["password"]; got != cfg.Secrets.Harbor.RegistryPassword {
		t.Fatalf("registry password = %#v, want %q", got, cfg.Secrets.Harbor.RegistryPassword)
	}
	if got := registry["htpasswdString"]; got != "" {
		t.Fatalf("registry htpasswdString = %#v, want empty string", got)
	}
	database := values["database"].(map[string]any)["internal"].(map[string]any)
	if got := database["password"]; got != cfg.Secrets.Harbor.DatabasePassword {
		t.Fatalf("database password = %#v, want %q", got, cfg.Secrets.Harbor.DatabasePassword)
	}
}

func TestRenderSingleServiceWithEncryptionWithoutSensitiveFilesNeedsNoAgeKey(t *testing.T) {
	cfg := newDefault("non-sensitive-no-key")
	cfg.OpenCenter.GitOps.Repository.LocalDir = t.TempDir()
	cfg.Secrets.SopsAgeKeyFile = ""
	cfg.OpenCenter.Services["kafka-cluster"].(*configservices.DefaultServiceConfig).Enabled = true
	manager := sops.NewDefaultSOPSManager(
		crypto.NewDefaultKeyManager(filepath.Join(t.TempDir(), "keys")),
		sops.NewDefaultEncryptor(nil, nil),
		nil,
	)

	if err := RenderSingleServiceWithEncryption(context.Background(), cfg, "kafka-cluster", false, manager.EncryptServiceOverrideValues); err != nil {
		t.Fatalf("RenderSingleServiceWithEncryption() error = %v", err)
	}
	path := filepath.Join(cfg.GitDir(), "applications", "overlays", cfg.ClusterName(), "services", "kafka-cluster", "kustomization.yaml")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stat promoted non-sensitive service output: %v", err)
	}
}

func TestRenderSingleServiceWithEncryptionHarborWithoutAgeKeyPreservesOutput(t *testing.T) {
	cfg := newDefault("harbor-no-key")
	cfg.OpenCenter.GitOps.Repository.LocalDir = t.TempDir()
	cfg.Secrets.SopsAgeKeyFile = ""
	cfg.OpenCenter.Services["harbor"].(*configservices.HarborConfig).Enabled = true
	cfg.Secrets.Harbor = v2.HarborSecrets{
		AdminPassword:    "admin-plaintext-fixture",
		RegistryPassword: "registry-plaintext-fixture",
		DatabasePassword: "database-plaintext-fixture",
	}
	path := filepath.Join(cfg.GitDir(), "applications", "overlays", cfg.ClusterName(), "services", "harbor", "helm-values", "override-values.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create prior Harbor output directory: %v", err)
	}
	previous := []byte("harborAdminPassword: ENC[AES256_GCM,data:previous]\nsops:\n  mac: previous\n")
	if err := os.WriteFile(path, previous, 0o644); err != nil {
		t.Fatalf("write prior Harbor output: %v", err)
	}
	manager := sops.NewDefaultSOPSManager(
		crypto.NewDefaultKeyManager(filepath.Join(t.TempDir(), "keys")),
		sops.NewDefaultEncryptor(nil, nil),
		nil,
	)

	err := RenderSingleServiceWithEncryption(context.Background(), cfg, "harbor", false, manager.EncryptServiceOverrideValues)
	if err == nil || !strings.Contains(err.Error(), "No age encryption keys available") {
		t.Fatalf("RenderSingleServiceWithEncryption() error = %v, want no-age-key failure", err)
	}
	got, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("read preserved Harbor output: %v", readErr)
	}
	if string(got) != string(previous) {
		t.Fatalf("Harbor output changed after encryption failure:\ngot:  %q\nwant: %q", got, previous)
	}
}
