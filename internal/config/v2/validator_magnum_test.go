package v2

import (
	"strings"
	"testing"
)

func validMagnumV2Config() *Config {
	cfg := newValidV2TestConfig("magnum")
	cfg.OpenCenter.Infrastructure.Cloud = CloudConfig{
		Magnum: &MagnumCloudConfig{
			AuthURL:                     "https://keystone.example.com/v3",
			Region:                      "RegionOne",
			ProjectID:                   "project-id",
			ApplicationCredentialID:     "application-credential-id",
			ApplicationCredentialSecret: "application-credential-secret",
			ClusterTemplate:             "kubernetes-template",
		},
	}
	return cfg
}

func TestDefaultValidatorMagnumMinimalConfigPasses(t *testing.T) {
	if err := NewValidator().Validate(validMagnumV2Config()); err != nil {
		t.Fatalf("valid minimal Magnum configuration failed validation: %v", err)
	}
}

func TestDefaultValidatorMagnumRequiresCloudConfiguration(t *testing.T) {
	cfg := validMagnumV2Config()
	cfg.OpenCenter.Infrastructure.Cloud = CloudConfig{}

	err := NewValidator().Validate(cfg)
	if err == nil || !strings.Contains(err.Error(), "cloud.magnum") {
		t.Fatalf("expected missing cloud.magnum validation error, got %v", err)
	}
}

func TestDefaultValidatorMagnumRequiresTemplateAndCredentials(t *testing.T) {
	cfg := validMagnumV2Config()
	cfg.OpenCenter.Infrastructure.Cloud.Magnum.ClusterTemplate = ""
	cfg.OpenCenter.Infrastructure.Cloud.Magnum.ApplicationCredentialSecret = ""

	err := NewValidator().Validate(cfg)
	if err == nil || (!strings.Contains(err.Error(), "ApplicationCredential") && !strings.Contains(err.Error(), "application_credential") && !strings.Contains(err.Error(), "ClusterTemplate") && !strings.Contains(err.Error(), "cluster_template")) {
		t.Fatalf("expected Magnum credential or template validation error, got %v", err)
	}
}

func TestDefaultValidatorMagnumAcceptsPasswordAuthWithoutAppCreds(t *testing.T) {
	cfg := validMagnumV2Config()
	magnum := cfg.OpenCenter.Infrastructure.Cloud.Magnum
	magnum.ApplicationCredentialID = ""
	magnum.ApplicationCredentialSecret = ""
	magnum.Username = "svc-user"
	magnum.Password = "svc-pass"
	magnum.UserDomainName = "Default"

	if err := NewValidator().Validate(cfg); err != nil {
		t.Fatalf("complete username/password Magnum config failed validation: %v", err)
	}
}

func TestDefaultValidatorMagnumRejectsIncompletePasswordAuth(t *testing.T) {
	cfg := validMagnumV2Config()
	magnum := cfg.OpenCenter.Infrastructure.Cloud.Magnum
	magnum.ApplicationCredentialID = ""
	magnum.ApplicationCredentialSecret = ""
	magnum.Username = "svc-user"
	// Password intentionally omitted: username/password must be supplied together.

	err := NewValidator().Validate(cfg)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "password") {
		t.Fatalf("expected incomplete password-auth validation error, got %v", err)
	}
}

func TestDefaultValidatorMagnumRejectsNoAuthMethod(t *testing.T) {
	cfg := validMagnumV2Config()
	magnum := cfg.OpenCenter.Infrastructure.Cloud.Magnum
	magnum.ApplicationCredentialID = ""
	magnum.ApplicationCredentialSecret = ""
	// Neither application credentials nor username/password configured.

	err := NewValidator().Validate(cfg)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "username/password or application_credential") {
		t.Fatalf("expected missing-auth-method validation error, got %v", err)
	}
}
