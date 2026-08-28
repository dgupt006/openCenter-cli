package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

func harborRenderTestConfig(t *testing.T, gitDir string) *v2.Config {
	t.Helper()
	cfg, err := v2.NewV2Default("harbor-render", "kind")
	if err != nil {
		t.Fatalf("NewV2Default() error = %v", err)
	}
	cfg.OpenCenter.GitOps.Repository.LocalDir = gitDir
	cfg.OpenCenter.Services["harbor"].(*services.HarborConfig).Enabled = true
	cfg.Secrets.Harbor = v2.HarborSecrets{
		AdminPassword:    "admin-plaintext-fixture",
		RegistryPassword: "registry-plaintext-fixture",
		DatabasePassword: "database-plaintext-fixture",
	}
	return cfg
}

func TestRenderServicesOnlyEncryptsHarborBeforePromotion(t *testing.T) {
	cfg := harborRenderTestConfig(t, t.TempDir())
	originalEncryptor := encryptRenderedServiceOverrides
	defer func() { encryptRenderedServiceOverrides = originalEncryptor }()

	encryptRenderedServiceOverrides = func(_ context.Context, overlayPath string, _ *v2.Config) error {
		path := filepath.Join(overlayPath, "services", "harbor", "helm-values", "override-values.yaml")
		plain, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, secret := range []string{"admin-plaintext-fixture", "registry-plaintext-fixture", "database-plaintext-fixture"} {
			if !strings.Contains(string(plain), secret) {
				return fmt.Errorf("temporary Harbor values missing %q", secret)
			}
		}
		return os.WriteFile(path, []byte("harborAdminPassword: ENC[AES256_GCM,data:fixture]\nsops:\n  mac: ENC[AES256_GCM,data:fixture]\n"), 0o644)
	}

	cmd, _ := newTestCmd()
	if err := renderServicesOnly(cfg, false, false, cmd); err != nil {
		t.Fatalf("renderServicesOnly() error = %v", err)
	}

	path := filepath.Join(cfg.GitDir(), "applications", "overlays", cfg.ClusterName(), "services", "harbor", "helm-values", "override-values.yaml")
	promoted, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read promoted Harbor values: %v", err)
	}
	if !strings.Contains(string(promoted), "sops:") {
		t.Fatalf("promoted Harbor values lack SOPS metadata: %q", promoted)
	}
	for _, secret := range []string{"admin-plaintext-fixture", "registry-plaintext-fixture", "database-plaintext-fixture"} {
		if strings.Contains(string(promoted), secret) {
			t.Fatalf("promoted Harbor values contain plaintext secret %q: %q", secret, promoted)
		}
	}
}

func TestRenderSingleServiceEncryptionFailurePreservesHarborOutput(t *testing.T) {
	cfg := harborRenderTestConfig(t, t.TempDir())
	path := filepath.Join(cfg.GitDir(), "applications", "overlays", cfg.ClusterName(), "services", "harbor", "helm-values", "override-values.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create existing Harbor directory: %v", err)
	}
	previous := []byte("harborAdminPassword: ENC[AES256_GCM,data:previous]\nsops:\n  mac: previous\n")
	if err := os.WriteFile(path, previous, 0o644); err != nil {
		t.Fatalf("write existing Harbor values: %v", err)
	}

	originalEncryptor := encryptRenderedServiceOverrides
	defer func() { encryptRenderedServiceOverrides = originalEncryptor }()
	encryptRenderedServiceOverrides = func(_ context.Context, overlayPath string, _ *v2.Config) error {
		temporaryPath := filepath.Join(overlayPath, "services", "harbor", "helm-values", "override-values.yaml")
		plain, err := os.ReadFile(temporaryPath)
		if err != nil {
			return err
		}
		if !strings.Contains(string(plain), "admin-plaintext-fixture") {
			return fmt.Errorf("temporary Harbor values were not rendered")
		}
		return fmt.Errorf("fixture encryption failure")
	}

	cmd, _ := newTestCmd()
	err := renderSingleService(cfg, "harbor", true, false, cmd)
	if err == nil || !strings.Contains(err.Error(), "fixture encryption failure") {
		t.Fatalf("renderSingleService() error = %v, want encryption failure", err)
	}
	got, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("read preserved Harbor values: %v", readErr)
	}
	if string(got) != string(previous) {
		t.Fatalf("Harbor output changed after encryption failure:\ngot:  %q\nwant: %q", got, previous)
	}
}

func TestRenderServicesOnlyDryRunDoesNotInvokeEncryption(t *testing.T) {
	cfg := harborRenderTestConfig(t, t.TempDir())
	originalEncryptor := encryptRenderedServiceOverrides
	defer func() { encryptRenderedServiceOverrides = originalEncryptor }()
	called := false
	encryptRenderedServiceOverrides = func(context.Context, string, *v2.Config) error {
		called = true
		return nil
	}

	cmd, _ := newTestCmd()
	if err := renderServicesOnly(cfg, false, true, cmd); err != nil {
		t.Fatalf("renderServicesOnly(dry-run) error = %v", err)
	}
	if called {
		t.Fatal("dry-run invoked the encryption/rendering path")
	}
}

func fixtureSOPSOverrideEncryptor(_ context.Context, overlayPath string, _ *v2.Config) error {
	return filepath.Walk(overlayPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || info.Name() != "override-values.yaml" {
			return nil
		}
		normalized := filepath.ToSlash(path)
		if !strings.Contains(normalized, "/services/") && !strings.Contains(normalized, "/managed-services/") {
			return nil
		}
		return os.WriteFile(path, []byte("encrypted: ENC[AES256_GCM,data:fixture]\nsops:\n  mac: ENC[AES256_GCM,data:fixture]\n"), info.Mode())
	})
}
