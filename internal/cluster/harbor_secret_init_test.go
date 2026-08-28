package cluster

import (
	"context"
	"strings"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config"
	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	"gopkg.in/yaml.v3"
)

func TestInitServiceInitializeGeneratesDistinctHarborSecrets(t *testing.T) {
	base, err := v2.NewV2Default("harbor-init", "kind")
	if err != nil {
		t.Fatalf("NewV2Default() error = %v", err)
	}
	base.OpenCenter.Services["harbor"].(*services.HarborConfig).Enabled = true
	data, err := v2.MarshalPublicConfig(base)
	if err != nil {
		t.Fatalf("MarshalPublicConfig() error = %v", err)
	}
	var configMap map[string]any
	if err := yaml.Unmarshal(data, &configMap); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}

	pathResolver := paths.NewPathResolver(t.TempDir())
	validationEngine := setupValidationEngine(t)
	configManager, err := config.NewConfigManager("")
	if err != nil {
		t.Fatalf("NewConfigManager() error = %v", err)
	}
	initService := NewInitService(pathResolver, validationEngine, configManager)

	result, err := initService.Initialize(context.Background(), InitOptions{
		ClusterName:  "harbor-init",
		Organization: "test-org",
		Provider:     "kind",
		ConfigMap:    configMap,
		NoKeyGen:     true,
		NoGitInit:    true,
	})
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	secrets := result.Config.Secrets.Harbor
	if secrets.AdminPassword == v2.PlaceholderSecret || secrets.RegistryPassword == v2.PlaceholderSecret || secrets.DatabasePassword == v2.PlaceholderSecret {
		t.Fatalf("Initialize() left Harbor placeholders: %#v", secrets)
	}
	if secrets.AdminPassword == "" || secrets.RegistryPassword == "" || secrets.DatabasePassword == "" {
		t.Fatalf("Initialize() left Harbor credentials empty: %#v", secrets)
	}
	if secrets.AdminPassword == secrets.RegistryPassword || secrets.AdminPassword == secrets.DatabasePassword || secrets.RegistryPassword == secrets.DatabasePassword {
		t.Fatalf("Initialize() generated duplicate Harbor credentials: %#v", secrets)
	}

	callerYAML, err := yaml.Marshal(configMap)
	if err != nil {
		t.Fatalf("marshal caller config map: %v", err)
	}
	for _, generated := range []string{secrets.AdminPassword, secrets.RegistryPassword, secrets.DatabasePassword} {
		if !strings.Contains(string(callerYAML), generated) {
			t.Fatalf("Initialize() did not merge generated credential into caller map: %s", callerYAML)
		}
	}
}

func TestGenerateHarborSecretsIgnoresManagedHarbor(t *testing.T) {
	cfg, err := v2.NewV2Default("managed-harbor-init", "kind")
	if err != nil {
		t.Fatalf("NewV2Default() error = %v", err)
	}
	cfg.OpenCenter.Services["harbor"].(*services.HarborConfig).Enabled = false
	cfg.OpenCenter.ManagedServices = v2.ServiceMap{
		"harbor": &services.HarborConfig{BaseConfig: services.BaseConfig{Enabled: true}},
	}
	original := cfg.Secrets.Harbor
	configMap := make(map[string]any)

	service := &InitService{}
	if err := service.generateHarborSecrets(cfg, configMap); err != nil {
		t.Fatalf("generateHarborSecrets() error = %v", err)
	}
	if cfg.Secrets.Harbor != original {
		t.Fatalf("managed Harbor credentials changed: got %#v, want %#v", cfg.Secrets.Harbor, original)
	}
	if len(configMap) != 0 {
		t.Fatalf("managed Harbor mutated config map: %#v", configMap)
	}
}
