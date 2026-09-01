package v2

import (
	"strings"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
)

func TestValidateForDeploymentRejectsMimirSwiftCredentialPlaceholder(t *testing.T) {
	cfg := validReadinessConfig(t, "kind")
	cfg.OpenCenter.Services["mimir"].(*services.DefaultServiceConfig).Enabled = true
	cfg.Secrets.Mimir.SwiftApplicationCredentialSecret = PlaceholderSecret

	err := ValidateForDeployment(cfg)
	if err == nil || !strings.Contains(err.Error(), "secrets.mimir.swift_application_credential_secret") {
		t.Fatalf("ValidateForDeployment() error = %v, want Mimir Swift credential path", err)
	}
}

func TestSwiftApplicationCredentialAccessorsFallBackToGlobalApplicationSecret(t *testing.T) {
	cfg := validReadinessConfig(t, "kind")
	cfg.Secrets.Global.AWS.Application.SecretAccessKey = "global-app-secret"
	cfg.Secrets.Mimir.SwiftApplicationCredentialSecret = ""
	cfg.Secrets.Loki.SwiftApplicationCredentialSecret = ""
	cfg.Secrets.Tempo.SwiftApplicationCredentialSecret = ""

	for name, got := range map[string]string{
		"Mimir": cfg.GetMimirSwiftApplicationCredentialSecret(),
		"Loki":  cfg.GetLokiSwiftApplicationCredentialSecret(),
		"Tempo": cfg.GetTempoSwiftApplicationCredentialSecret(),
	} {
		if got != "global-app-secret" {
			t.Errorf("Get%sSwiftApplicationCredentialSecret() = %q, want global-app-secret", name, got)
		}
	}
}
