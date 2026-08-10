package cluster

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReconcileSopsAgeSecret_Success(t *testing.T) {
	// Write a fake age key file.
	keyDir := t.TempDir()
	keyPath := filepath.Join(keyDir, "demo-key.txt")
	keyContent := "# public key: age1ql3z7hjy54pw3hyww5ayyfg7zqgvc7w3j2elw8zmrj2kg5sfn9aqmcac8p\nAGE-SECRET-KEY-1QFNYJ49KCKQ4DLXHYPHXF4RKA78S4RAGRFNUL3\n"
	require.NoError(t, os.WriteFile(keyPath, []byte(keyContent), 0o600))

	runner := &fakeLifecycleRunner{onRun: func(dir string, env map[string]string, name string, args ...string) ([]byte, error) {
		if name == "kubectl" && strings.Contains(strings.Join(args, " "), "create secret generic") {
			return []byte("apiVersion: v1\nkind: Secret\nmetadata:\n  name: sops-age\n"), nil
		}
		return nil, nil
	}}

	err := reconcileSopsAgeSecret(context.Background(), keyPath, "/tmp/kubeconfig", runner)
	require.NoError(t, err)

	// Should have made exactly 2 kubectl calls: create (dry-run) + apply.
	require.Len(t, runner.calls, 2)

	// First call: kubectl create secret generic sops-age --from-file=...
	createCall := runner.calls[0]
	assert.Equal(t, "kubectl", createCall.name)
	createArgs := strings.Join(createCall.args, " ")
	assert.Contains(t, createArgs, "create secret generic sops-age")
	assert.Contains(t, createArgs, "--from-file=age.agekey=")
	assert.Contains(t, createArgs, "-n flux-system")
	assert.Contains(t, createArgs, "--dry-run=client")
	assert.Contains(t, createArgs, "-o yaml")
	assert.Contains(t, createArgs, "--kubeconfig /tmp/kubeconfig")

	// The key content must NOT appear in the command args (it's in a temp file).
	assert.NotContains(t, createArgs, "AGE-SECRET-KEY")

	// Second call: kubectl apply -f <manifest>
	applyCall := runner.calls[1]
	assert.Equal(t, "kubectl", applyCall.name)
	applyArgs := strings.Join(applyCall.args, " ")
	assert.Contains(t, applyArgs, "apply -f")
	assert.Contains(t, applyArgs, "--kubeconfig /tmp/kubeconfig")
}

func TestReconcileSopsAgeSecret_MissingKeyFile(t *testing.T) {
	runner := &fakeLifecycleRunner{}
	err := reconcileSopsAgeSecret(context.Background(), "/nonexistent/path/key.txt", "/tmp/kubeconfig", runner)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read SOPS Age key")
	assert.Contains(t, err.Error(), "opencenter cluster init")
	assert.Empty(t, runner.calls, "no kubectl calls should be made when key is missing")
}

func TestReconcileSopsAgeSecret_EmptyKeyPath(t *testing.T) {
	runner := &fakeLifecycleRunner{}
	err := reconcileSopsAgeSecret(context.Background(), "", "/tmp/kubeconfig", runner)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SOPS Age key path is empty")
	assert.Empty(t, runner.calls)
}

func TestReconcileSopsAgeSecret_EmptyKeyFile(t *testing.T) {
	keyDir := t.TempDir()
	keyPath := filepath.Join(keyDir, "empty-key.txt")
	require.NoError(t, os.WriteFile(keyPath, []byte(""), 0o600))

	runner := &fakeLifecycleRunner{}
	err := reconcileSopsAgeSecret(context.Background(), keyPath, "/tmp/kubeconfig", runner)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "is empty")
	assert.Empty(t, runner.calls)
}

func TestNewSopsAgeSecretStep_Metadata(t *testing.T) {
	step := newSopsAgeSecretStep("/path/to/key.txt", "/path/to/kubeconfig", &fakeLifecycleRunner{})

	assert.Equal(t, sopsAgeSecretStepID, step.ID)
	assert.Equal(t, "Reconcile SOPS Age decryption key for Flux", step.Description)
	assert.Equal(t, sopsAgeSecretStepID, step.Plan.ID)
	assert.Contains(t, step.Plan.Reads, "/path/to/key.txt")
	assert.Contains(t, step.Plan.Reads, "/path/to/kubeconfig")
	assert.Contains(t, step.Plan.Writes[0], "sops-age")
	assert.Contains(t, step.Plan.Writes[0], "flux-system")
}

// TestReconcileSopsAgeSecret_KeyNotExposedInArgs verifies that the Age
// private key content never leaks into kubectl command arguments — it must
// only be referenced by temp-file path.
func TestReconcileSopsAgeSecret_KeyNotExposedInArgs(t *testing.T) {
	keyDir := t.TempDir()
	keyPath := filepath.Join(keyDir, "demo-key.txt")
	secretKey := "AGE-SECRET-KEY-1QFNYJ49KCKQ4DLXHYPHXF4RKA78S4RAGRFNUL3"
	keyContent := "# public key: age1ql3z7hjy54pw3hyww5ayyfg7zqgvc7w3j2elw8zmrj2kg5sfn9aqmcac8p\n" + secretKey + "\n"
	require.NoError(t, os.WriteFile(keyPath, []byte(keyContent), 0o600))

	runner := &fakeLifecycleRunner{onRun: func(dir string, env map[string]string, name string, args ...string) ([]byte, error) {
		if name == "kubectl" && strings.Contains(strings.Join(args, " "), "create secret generic") {
			return []byte("apiVersion: v1\nkind: Secret\n"), nil
		}
		return nil, nil
	}}

	require.NoError(t, reconcileSopsAgeSecret(context.Background(), keyPath, "/tmp/kubeconfig", runner))

	for i, call := range runner.calls {
		allArgs := strings.Join(call.args, " ")
		assert.NotContains(t, allArgs, secretKey,
			"call %d (%s) must not contain the Age secret key in arguments", i, call.name)
	}
}
