package v2

import (
	"strings"
	"testing"
)

func TestDefaultStorageClassRequiresDNS1123AtWriteValidationBoundary(t *testing.T) {
	cfg := validReadinessConfig(t, "kind")

	for _, valid := range []string{"standard", "csi-cinder-sc-delete", "storage.example.com"} {
		cfg.OpenCenter.Infrastructure.Storage.DefaultStorageClass = valid
		if err := NewValidator().ValidateSchema(cfg); err != nil {
			t.Errorf("ValidateSchema() rejected valid storage class %q: %v", valid, err)
		}
	}

	for _, invalid := range []string{"Performance", "fast_storage", "-fast", "fast-", "fast..storage", strings.Repeat("a", 64)} {
		cfg.OpenCenter.Infrastructure.Storage.DefaultStorageClass = invalid
		err := NewValidator().ValidateSchema(cfg)
		if err == nil || !strings.Contains(err.Error(), "DefaultStorageClass") {
			t.Errorf("ValidateSchema() error for %q = %v, want DefaultStorageClass DNS-1123 failure", invalid, err)
		}
	}
}

func TestDefaultStorageClassRequiresDNS1123AtGenerationBoundary(t *testing.T) {
	cfg := validReadinessConfig(t, "kind")
	cfg.OpenCenter.Infrastructure.Storage.DefaultStorageClass = "Performance"

	err := ValidateForGeneration(cfg)
	if err == nil || !strings.Contains(err.Error(), "DefaultStorageClass") {
		t.Fatalf("ValidateForGeneration() error = %v, want storage class DNS-1123 failure", err)
	}
}
