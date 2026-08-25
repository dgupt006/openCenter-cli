package sops

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/util/crypto"
	utilerrors "github.com/opencenter-cloud/opencenter-cli/internal/util/errors"
)

func TestEncryptServiceOverrideValuesEncryptsSensitiveOverrideValues(t *testing.T) {
	if _, err := exec.LookPath("age-keygen"); err != nil {
		t.Skip("age-keygen is not installed")
	}
	if _, err := exec.LookPath("sops"); err != nil {
		t.Skip("sops is not installed")
	}

	tmpDir := t.TempDir()
	keyOutput, err := exec.Command("age-keygen").Output()
	if err != nil {
		t.Fatalf("generate age fixture: %v", err)
	}
	keyPath := filepath.Join(tmpDir, "age-key.txt")
	if err := os.WriteFile(keyPath, keyOutput, 0o600); err != nil {
		t.Fatalf("write age fixture: %v", err)
	}

	overridePath := filepath.Join(tmpDir, "overlay", "services", "headlamp", "helm-values", "override-values.yaml")
	plaintext := "clientSecret: fixture-headlamp-secret\n"
	if err := os.MkdirAll(filepath.Dir(overridePath), 0o755); err != nil {
		t.Fatalf("create override directory: %v", err)
	}
	if err := os.WriteFile(overridePath, []byte(plaintext), 0o644); err != nil {
		t.Fatalf("write plaintext override: %v", err)
	}

	cfg := newSOPSTestConfig("sensitive-overrides", "baremetal", keyPath)
	manager := NewSOPSManager()
	if err := manager.EncryptServiceOverrideValues(context.Background(), filepath.Join(tmpDir, "overlay"), cfg); err != nil {
		t.Fatalf("EncryptServiceOverrideValues() error = %v", err)
	}

	data, err := os.ReadFile(overridePath)
	if err != nil {
		t.Fatalf("read encrypted override: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "sops:") {
		t.Fatalf("encrypted override missing SOPS metadata: %q", content)
	}
	if strings.Contains(content, "fixture-headlamp-secret") {
		t.Fatalf("encrypted override contains fixture plaintext: %q", content)
	}
}

func TestEncryptServiceOverrideValuesOnlyEncryptsSelectedOverrides(t *testing.T) {
	tmpDir := t.TempDir()
	overlayPath := filepath.Join(tmpDir, "overlay")
	cfg := newSOPSTestConfig("service-overrides", "vsphere", "")

	for _, relPath := range overlayFilesToEncrypt(cfg) {
		path := filepath.Join(overlayPath, relPath)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("create directory for %s: %v", relPath, err)
		}
		if err := os.WriteFile(path, []byte("fixture: plaintext\n"), 0o644); err != nil {
			t.Fatalf("write fixture %s: %v", relPath, err)
		}
	}

	encryptor := &recordingEncryptor{}
	manager := NewDefaultSOPSManager(recordingKeyManager{}, encryptor, nil)
	if err := manager.EncryptServiceOverrideValues(context.Background(), overlayPath, cfg); err != nil {
		t.Fatalf("EncryptServiceOverrideValues() error = %v", err)
	}

	var got []string
	for _, call := range encryptor.calls {
		got = append(got, filepath.ToSlash(call.path[len(overlayPath)+1:]))
		if call.config.EncryptedRegex != ".*" {
			t.Fatalf("encrypted regex for %s = %q, want .*", call.path, call.config.EncryptedRegex)
		}
	}
	want := serviceOverrideValuesFilesToEncrypt(cfg)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("encrypted files = %v, want %v", got, want)
	}

	for _, relPath := range []string{
		"flux-system/gotk-sync.yaml",
		"managed-services/sources/base-repo.yaml",
		"secrets/vsphere-credentials.yaml",
		"customer-managed/services/cloud-provider-vsphere/secret.yaml",
	} {
		for _, call := range encryptor.calls {
			if filepath.ToSlash(call.path[len(overlayPath)+1:]) == relPath {
				t.Fatalf("non-override file %s was selected for encryption", relPath)
			}
		}
	}
}

type recordingEncryptionCall struct {
	path   string
	config EncryptionConfig
}

type recordingEncryptor struct {
	Encryptor
	calls []recordingEncryptionCall
}

func (e *recordingEncryptor) EncryptFile(_ context.Context, path string, config EncryptionConfig) error {
	e.calls = append(e.calls, recordingEncryptionCall{path: path, config: config})
	return nil
}

func (e *recordingEncryptor) IsFileEncrypted(string) (bool, error) { return true, nil }

type recordingKeyManager struct {
	crypto.KeyManager
}

func (recordingKeyManager) ListAgeKeys() ([]string, error) { return []string{"fixture"}, nil }
func (recordingKeyManager) LoadAgeKey(string) (*crypto.AgeKeyPair, error) {
	return &crypto.AgeKeyPair{PublicKey: "age1fixture"}, nil
}

func TestEncryptOverlayFilesConfiguredKeyFailureIsFailClosed(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := newSOPSTestConfig("configured-key-failure", "baremetal", filepath.Join(tmpDir, "missing-age-key.txt"))
	manager := NewSOPSManager()

	err := manager.EncryptOverlayFiles(context.Background(), filepath.Join(tmpDir, "overlay"), cfg)
	if err == nil {
		t.Fatal("EncryptOverlayFiles() returned nil for a missing configured key")
	}
	structured, ok := err.(*utilerrors.StructuredError)
	if !ok {
		t.Fatalf("EncryptOverlayFiles() error type = %T, want *StructuredError", err)
	}
	if structured.Message != "Failed to load configured age key" {
		t.Fatalf("EncryptOverlayFiles() error message = %q, want configured-key failure", structured.Message)
	}
}

func TestOverlayFilesToEncrypt(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		want     []string
	}{
		{
			name:     "provider without additional secret files",
			provider: "baremetal",
			want: []string{
				"flux-system/gotk-sync.yaml",
				"managed-services/sources/base-repo.yaml",
				// Service override-values with credentials (always included)
				"services/loki/helm-values/override-values.yaml",
				"services/tempo/helm-values/override-values.yaml",
				"services/mimir/helm-values/override-values.yaml",
				"services/headlamp/helm-values/override-values.yaml",
				"services/harbor/helm-values/override-values.yaml",
			},
		},
		{
			name:     "OpenStack",
			provider: "openstack",
			want: []string{
				"flux-system/gotk-sync.yaml",
				"managed-services/sources/base-repo.yaml",
				"secrets/openstack-credentials.yaml",
				// OpenStack-specific service override-values with credentials
				"services/openstack-ccm/helm-values/override-values.yaml",
				"services/openstack-csi/helm-values/override-values.yaml",
				// Service override-values with credentials (always included)
				"services/loki/helm-values/override-values.yaml",
				"services/tempo/helm-values/override-values.yaml",
				"services/mimir/helm-values/override-values.yaml",
				"services/headlamp/helm-values/override-values.yaml",
				"services/harbor/helm-values/override-values.yaml",
			},
		},
		{
			name:     "vSphere",
			provider: "vsphere",
			want: []string{
				"flux-system/gotk-sync.yaml",
				"managed-services/sources/base-repo.yaml",
				"secrets/vsphere-credentials.yaml",
				"customer-managed/services/cloud-provider-vsphere/secret.yaml",
				// Service override-values with credentials (always included)
				"services/loki/helm-values/override-values.yaml",
				"services/tempo/helm-values/override-values.yaml",
				"services/mimir/helm-values/override-values.yaml",
				"services/headlamp/helm-values/override-values.yaml",
				"services/harbor/helm-values/override-values.yaml",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := newSOPSTestConfig("test-cluster", "baremetal", "")
			cfg.OpenCenter.Infrastructure.Provider = tt.provider

			if got := overlayFilesToEncrypt(cfg); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("overlayFilesToEncrypt() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestServiceOverrideValuesFilesToEncrypt(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		want     []string
	}{
		{
			name:     "OpenStack includes openstack-ccm and openstack-csi",
			provider: "openstack",
			want: []string{
				"services/openstack-ccm/helm-values/override-values.yaml",
				"services/openstack-csi/helm-values/override-values.yaml",
				"services/loki/helm-values/override-values.yaml",
				"services/tempo/helm-values/override-values.yaml",
				"services/mimir/helm-values/override-values.yaml",
				"services/headlamp/helm-values/override-values.yaml",
				"services/harbor/helm-values/override-values.yaml",
			},
		},
		{
			name:     "non-OpenStack excludes openstack-ccm and openstack-csi",
			provider: "baremetal",
			want: []string{
				"services/loki/helm-values/override-values.yaml",
				"services/tempo/helm-values/override-values.yaml",
				"services/mimir/helm-values/override-values.yaml",
				"services/headlamp/helm-values/override-values.yaml",
				"services/harbor/helm-values/override-values.yaml",
			},
		},
		{
			name:     "vSphere excludes openstack-ccm and openstack-csi",
			provider: "vsphere",
			want: []string{
				"services/loki/helm-values/override-values.yaml",
				"services/tempo/helm-values/override-values.yaml",
				"services/mimir/helm-values/override-values.yaml",
				"services/headlamp/helm-values/override-values.yaml",
				"services/harbor/helm-values/override-values.yaml",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := newSOPSTestConfig("test-cluster", tt.provider, "")

			got := serviceOverrideValuesFilesToEncrypt(cfg)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("serviceOverrideValuesFilesToEncrypt() = %v, want %v", got, tt.want)
			}
		})
	}
}
