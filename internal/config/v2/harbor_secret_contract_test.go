package v2

import (
	"strings"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
)

func TestNewV2DefaultHarborSecretsUseDeterministicPlaceholders(t *testing.T) {
	first, err := NewV2Default("harbor-defaults", "kind")
	if err != nil {
		t.Fatalf("NewV2Default() error = %v", err)
	}
	second, err := NewV2Default("harbor-defaults", "kind")
	if err != nil {
		t.Fatalf("second NewV2Default() error = %v", err)
	}

	if first.Secrets.Harbor != (HarborSecrets{
		AdminPassword:     PlaceholderSecret,
		RegistryPassword:  PlaceholderSecret,
		DatabasePassword:  PlaceholderSecret,
		S3AccessKeyID:     PlaceholderSecret,
		S3SecretAccessKey: PlaceholderSecret,
	}) {
		t.Fatalf("Harbor defaults = %#v, want all CHANGEME", first.Secrets.Harbor)
	}
	if first.Secrets.Harbor != second.Secrets.Harbor {
		t.Fatalf("Harbor defaults are not deterministic: %#v != %#v", first.Secrets.Harbor, second.Secrets.Harbor)
	}
}

func TestHarborSecretValidationDependsOnServiceEnabled(t *testing.T) {
	cfg := validReadinessConfig(t, "kind")
	harbor := cfg.OpenCenter.Services["harbor"].(*services.HarborConfig)
	harbor.Enabled = true
	cfg.Secrets.Harbor = HarborSecrets{
		AdminPassword:     PlaceholderSecret,
		RegistryPassword:  "",
		DatabasePassword:  PlaceholderSecret,
		S3AccessKeyID:     PlaceholderSecret,
		S3SecretAccessKey: PlaceholderSecret,
	}

	report := ValidateReadiness(cfg)
	for _, path := range []string{
		"secrets.harbor.admin_password",
		"secrets.harbor.registry_password",
		"secrets.harbor.database_password",
		"secrets.harbor.s3_access_key_id",
		"secrets.harbor.s3_secret_access_key",
	} {
		assertIssue(t, report, SeverityError, CategoryServices, path)
	}

	err := ValidateForDeployment(cfg)
	if err == nil {
		t.Fatal("ValidateForDeployment() accepted enabled Harbor with missing secrets")
	}
	for _, path := range []string{
		"secrets.harbor.admin_password",
		"secrets.harbor.registry_password",
		"secrets.harbor.database_password",
		"secrets.harbor.s3_access_key_id",
		"secrets.harbor.s3_secret_access_key",
	} {
		if !strings.Contains(err.Error(), path) {
			t.Fatalf("ValidateForDeployment() error = %v, missing %q", err, path)
		}
	}

	harbor.Enabled = false
	report = ValidateReadiness(cfg)
	for _, path := range []string{
		"secrets.harbor.admin_password",
		"secrets.harbor.registry_password",
		"secrets.harbor.database_password",
		"secrets.harbor.s3_access_key_id",
		"secrets.harbor.s3_secret_access_key",
	} {
		assertNoIssue(t, report, path)
	}
	if err := ValidateForDeployment(cfg); err != nil {
		t.Fatalf("ValidateForDeployment() rejected disabled Harbor placeholders: %v", err)
	}
}

func TestValidateHarborForDeploymentChecksRegularHarborSecrets(t *testing.T) {
	cfg := validReadinessConfig(t, "kind")
	cfg.OpenCenter.Services["harbor"] = &services.HarborConfig{BaseConfig: services.BaseConfig{Enabled: true}}
	cfg.Secrets.Harbor = HarborSecrets{
		AdminPassword:     PlaceholderSecret,
		RegistryPassword:  "registry-configured",
		DatabasePassword:  "database-configured",
		S3AccessKeyID:     "ec2-access-key",
		S3SecretAccessKey: "ec2-secret-key",
	}

	err := ValidateHarborForDeployment(cfg)
	if err == nil || !strings.Contains(err.Error(), "secrets.harbor.admin_password") {
		t.Fatalf("ValidateHarborForDeployment() error = %v", err)
	}

	cfg.Secrets.Harbor.AdminPassword = "admin-configured"
	if err := ValidateHarborForDeployment(cfg); err != nil {
		t.Fatalf("ValidateHarborForDeployment() rejected configured regular Harbor: %v", err)
	}
}

func TestValidateHarborForDeploymentRejectsManagedHarbor(t *testing.T) {
	cfg := validReadinessConfig(t, "kind")
	cfg.OpenCenter.Services["harbor"].(*services.HarborConfig).Enabled = false
	cfg.OpenCenter.ManagedServices = ServiceMap{
		"harbor": &services.HarborConfig{BaseConfig: services.BaseConfig{Enabled: true}},
	}
	cfg.Secrets.Harbor = HarborSecrets{
		AdminPassword:     "admin-configured",
		RegistryPassword:  "registry-configured",
		DatabasePassword:  "database-configured",
		S3AccessKeyID:     "ec2-access-key",
		S3SecretAccessKey: "ec2-secret-key",
	}

	err := ValidateHarborForDeployment(cfg)
	if err == nil || !strings.Contains(err.Error(), "managed Harbor is not supported") {
		t.Fatalf("ValidateHarborForDeployment() error = %v", err)
	}
}
