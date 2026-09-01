package v2

import (
	"strings"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
)

func TestValidateHarborEndpointRequiredOnlyWhenEnabled(t *testing.T) {
	base := services.HarborConfig{
		BaseConfig:           services.BaseConfig{Enabled: true},
		StorageType:          "s3",
		RegistryVolumeSize:   100,
		JobserviceVolumeSize: 5,
		DatabaseVolumeSize:   10,
		RedisVolumeSize:      5,
		TrivyVolumeSize:      5,
	}
	if err := ValidateHarborConfig(&base); err == nil || !strings.Contains(err.Error(), "s3_endpoint") {
		t.Fatalf("enabled Harbor validation error = %v, want missing endpoint", err)
	}

	base.Enabled = false
	if err := ValidateHarborConfig(&base); err != nil {
		t.Fatalf("disabled default Harbor validation error = %v", err)
	}
}

func TestReadinessRequiresEndpointsForResolvedLokiTempoS3(t *testing.T) {
	cfg, err := NewV2Default("task16", "kind")
	if err != nil {
		t.Fatal(err)
	}
	cfg.OpenCenter.Infrastructure.Provider = "kind"
	loki := cfg.OpenCenter.Services["loki"].(*services.LokiConfig)
	loki.Enabled = true
	loki.StorageType = ""
	loki.S3Endpoint = ""
	cfg.Secrets.Loki.S3AccessKeyID = "loki-access"
	cfg.Secrets.Loki.S3SecretAccessKey = "loki-secret"
	tempo := cfg.OpenCenter.Services["tempo"].(*services.TempoConfig)
	tempo.Enabled = true
	tempo.StorageType = ""
	tempo.S3Endpoint = ""
	cfg.Secrets.Tempo.AccessKey = "tempo-access"
	cfg.Secrets.Tempo.SecretKey = "tempo-secret"

	report := ValidateReadiness(cfg)
	for _, path := range []string{
		"opencenter.services.loki.s3_endpoint",
		"opencenter.services.tempo.s3_endpoint",
	} {
		found := false
		for _, issue := range report.Issues {
			if issue.Path == path {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("readiness did not require endpoint %q: %#v", path, report.Issues)
		}
	}
}
