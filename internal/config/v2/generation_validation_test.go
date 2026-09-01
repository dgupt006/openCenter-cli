package v2

import (
	"strings"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
)

func TestValidateGenerationRejectsMissingEnabledServiceDependency(t *testing.T) {
	cfg := validReadinessConfig(t, "kind")
	cfg.OpenCenter.Services["keycloak"].(*services.KeycloakConfig).Enabled = false

	report := ValidateGeneration(cfg)

	if report.Valid {
		t.Fatalf("ValidateGeneration() accepted a missing service dependency: %#v", report)
	}
	if !generationReportContains(report, "headlamp", "keycloak") {
		t.Fatalf("generation report = %#v, want headlamp/keycloak dependency error", report)
	}
}

func TestValidateGenerationRejectsSelectedBackendPlaceholderSecret(t *testing.T) {
	cfg := validReadinessConfig(t, "kind")
	cfg.OpenCenter.Services["loki"].(*services.LokiConfig).StorageType = "s3"
	cfg.Secrets.Loki.S3AccessKeyID = PlaceholderSecret
	cfg.Secrets.Loki.S3SecretAccessKey = "loki-secret"

	report := ValidateGeneration(cfg)

	if report.Valid {
		t.Fatalf("ValidateGeneration() accepted a selected backend placeholder: %#v", report)
	}
	if !generationReportContains(report, "secrets.loki.s3_access_key_id", "access key") {
		t.Fatalf("generation report = %#v, want selected backend placeholder error", report)
	}
}

func TestValidateGenerationRejectsStaticProviderCapabilityIncompatibility(t *testing.T) {
	cfg := validReadinessConfig(t, "openstack")
	cfg.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.InstallMethod = "kubespray"

	report := ValidateGeneration(cfg)

	if report.Valid {
		t.Fatalf("ValidateGeneration() accepted an incompatible static capability: %#v", report)
	}
	if !generationReportContains(report, "network_plugin.calico.install_method", "kubespray") {
		t.Fatalf("generation report = %#v, want network-plugin compatibility error", report)
	}
}

func TestValidateGenerationAcceptsValidOfflineConfiguration(t *testing.T) {
	cfg := validReadinessConfig(t, "kind")
	cfg.OpenCenter.Infrastructure.Compute.WorkerCount = 3

	report := ValidateGeneration(cfg)

	if !report.Valid {
		t.Fatalf("ValidateGeneration() rejected valid configuration: %s", renderGenerationIssues(report.Issues))
	}
	if err := ValidateForGeneration(cfg); err != nil {
		t.Fatalf("ValidateForGeneration() error = %v", err)
	}
}

func generationReportContains(report GenerationValidationReport, terms ...string) bool {
	for _, issue := range report.Issues {
		text := strings.ToLower(issue.Path + " " + issue.Message)
		matched := true
		for _, term := range terms {
			if !strings.Contains(text, strings.ToLower(term)) {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}

func renderGenerationIssues(issues []ValidationIssue) string {
	var b strings.Builder
	for _, issue := range issues {
		b.WriteString(string(issue.Severity))
		b.WriteString(" ")
		b.WriteString(issue.Path)
		b.WriteString(": ")
		b.WriteString(issue.Message)
		b.WriteString("\n")
	}
	return b.String()
}
