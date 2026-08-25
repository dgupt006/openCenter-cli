package secrets

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writePhase6Fixture(t *testing.T, tmpDir, cluster, recipients, keyFile string) string {
	t.Helper()

	repoDir := filepath.Join(tmpDir, "phase6-repo")
	config := `schema_version: "2.0"
opencenter:
  cluster:
    cluster_name: ` + cluster + `
  gitops:
    git_dir: ` + repoDir + "\n"
	if keyFile != "" {
		config += "secrets:\n  sops_age_key_file: " + keyFile + "\n"
	}
	writeRotationTestConfig(t, cluster, config)

	overlayPath := filepath.Join(repoDir, "applications", "overlays", cluster)
	require.NoError(t, os.MkdirAll(overlayPath, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(overlayPath, ".sops.yaml"), []byte(`creation_rules:
  - path_regex: .*\.yaml$
    age: `+recipients+"\n"), 0o644))
	return overlayPath
}

func registerPhase6Key(t *testing.T, ctx context.Context, registry *MockKeyRegistry, cluster, fingerprint string, primary bool) {
	t.Helper()
	require.NoError(t, registry.RegisterKey(ctx, KeyEntry{
		Cluster:     cluster,
		KeyType:     KeyTypeAge,
		Fingerprint: fingerprint,
		PublicKey:   fingerprint,
		CreatedAt:   time.Now(),
		Status:      KeyStatusActive,
		Primary:     primary,
	}))
}

func TestPhase6RevocationRemainingOperatorKeySucceeds(t *testing.T) {
	revoker, registry, _, tmpDir, cleanup := setupTestRevoker(t)
	defer cleanup()
	ctx := context.Background()
	cluster := "phase6-remaining-key"
	keyFile := filepath.Join(tmpDir, "operator-age.txt")
	require.NoError(t, os.WriteFile(keyFile, []byte("# public key: age1keep\nAGE-SECRET-KEY-TEST\n"), 0o600))
	t.Setenv("SOPS_AGE_KEY_FILE", keyFile)
	writePhase6Fixture(t, tmpDir, cluster, "age1keep,age1revoke", "")
	registerPhase6Key(t, ctx, registry, cluster, "age1keep", true)
	registerPhase6Key(t, ctx, registry, cluster, "age1revoke", false)

	result, err := revoker.RevokeByFingerprint(ctx, RevokeOptions{
		Cluster: cluster, Fingerprint: "age1revoke",
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"age1revoke"}, result.RevokedKeys)
}

func TestPhase6RevocationMatchingOperatorKeyFailsWithoutMutation(t *testing.T) {
	revoker, registry, _, tmpDir, cleanup := setupTestRevoker(t)
	defer cleanup()
	ctx := context.Background()
	cluster := "phase6-matching-key"
	keyFile := filepath.Join(tmpDir, "operator-age.txt")
	require.NoError(t, os.WriteFile(keyFile, []byte("# public key: age1revoke\nAGE-SECRET-KEY-TEST\n"), 0o600))
	t.Setenv("SOPS_AGE_KEY_FILE", keyFile)
	overlayPath := writePhase6Fixture(t, tmpDir, cluster, "age1keep,age1revoke", "")
	registerPhase6Key(t, ctx, registry, cluster, "age1keep", true)
	registerPhase6Key(t, ctx, registry, cluster, "age1revoke", false)
	sopsPath := filepath.Join(overlayPath, ".sops.yaml")
	before, err := os.ReadFile(sopsPath)
	require.NoError(t, err)

	_, err = revoker.RevokeByFingerprint(ctx, RevokeOptions{
		Cluster: cluster, Fingerprint: "age1revoke",
	})
	var unreachableErr *ErrNoReachablePrivateKey
	require.ErrorAs(t, err, &unreachableErr)
	assert.Contains(t, unreachableErr.Error(), keyFile)
	after, readErr := os.ReadFile(sopsPath)
	require.NoError(t, readErr)
	assert.Equal(t, before, after)
}

func TestPhase6RevocationWithoutKeyFileSignalIsNotBlocked(t *testing.T) {
	revoker, registry, _, tmpDir, cleanup := setupTestRevoker(t)
	defer cleanup()
	t.Setenv("SOPS_AGE_KEY_FILE", "")
	ctx := context.Background()
	cluster := "phase6-no-key-signal"
	writePhase6Fixture(t, tmpDir, cluster, "age1keep,age1revoke", "")
	registerPhase6Key(t, ctx, registry, cluster, "age1keep", true)
	registerPhase6Key(t, ctx, registry, cluster, "age1revoke", false)

	_, err := revoker.RevokeByFingerprint(ctx, RevokeOptions{
		Cluster: cluster, Fingerprint: "age1revoke", DryRun: true,
	})
	require.NoError(t, err)
}

func TestPhase6RevocationOfPrimaryFails(t *testing.T) {
	revoker, registry, _, tmpDir, cleanup := setupTestRevoker(t)
	defer cleanup()
	ctx := context.Background()
	cluster := "phase6-primary"
	writePhase6Fixture(t, tmpDir, cluster, "age1primary,age1other", "")
	registerPhase6Key(t, ctx, registry, cluster, "age1primary", true)
	registerPhase6Key(t, ctx, registry, cluster, "age1other", false)

	_, err := revoker.RevokeByFingerprint(ctx, RevokeOptions{
		Cluster: cluster, Fingerprint: "age1primary", DryRun: true,
	})
	var primaryErr *ErrPrimaryKeyRevocation
	require.ErrorAs(t, err, &primaryErr)
	assert.Contains(t, primaryErr.Error(), "EmergencyRevoke")
}

type phase6EmergencyRotator struct {
	*MockKeyRotator
	registry *MockKeyRegistry
}

func (r *phase6EmergencyRotator) RotateAgeKey(ctx context.Context, opts RotateOptions) (*RotationResult, error) {
	keys, err := r.registry.ListKeys(ctx, opts.Cluster)
	if err != nil {
		return nil, err
	}
	for _, key := range keys {
		if key.KeyType == KeyTypeAge && key.Status == KeyStatusActive && key.Primary {
			key.Primary = false
			if err := r.registry.UpdateKey(ctx, key); err != nil {
				return nil, err
			}
		}
	}
	newFingerprint := "age1replacement"
	if err := r.registry.RegisterKey(ctx, KeyEntry{
		Cluster: opts.Cluster, KeyType: KeyTypeAge,
		Fingerprint: newFingerprint, PublicKey: newFingerprint,
		Status: KeyStatusActive, Primary: true,
	}); err != nil {
		return nil, err
	}
	return &RotationResult{
		OldFingerprint: "age1compromised",
		NewFingerprint: newFingerprint,
		DualKeyActive:  true,
	}, nil
}

func TestPhase6EmergencyRevokeBypassesPrimaryGuard(t *testing.T) {
	_, registry, manager, tmpDir, cleanup := setupTestRevoker(t)
	defer cleanup()
	ctx := context.Background()
	cluster := "phase6-emergency"
	overlayPath := writePhase6Fixture(t, tmpDir, cluster, "age1compromised", "")
	registerPhase6Key(t, ctx, registry, cluster, "age1compromised", true)
	rotator := &phase6EmergencyRotator{MockKeyRotator: &MockKeyRotator{}, registry: registry}
	revoker := NewDefaultKeyRevoker(registry, rotator, manager, nil, nil)

	result, err := revoker.EmergencyRevoke(ctx, cluster, "age1compromised")
	require.NoError(t, err)
	assert.Equal(t, "age1replacement", result.NewPrimaryKey)
	assert.Contains(t, result.RevokedKeys, "age1compromised")
	data, readErr := os.ReadFile(filepath.Join(overlayPath, ".sops.yaml"))
	require.NoError(t, readErr)
	assert.NotContains(t, string(data), "age1compromised")
	assert.Contains(t, string(data), "age1replacement")
}

func TestPhase6RemainingSetIsPassedToMutation(t *testing.T) {
	revoker, registry, _, tmpDir, cleanup := setupTestRevoker(t)
	defer cleanup()
	ctx := context.Background()
	cluster := "phase6-shared-set"
	writePhase6Fixture(t, tmpDir, cluster, "age1keep,age1revoke", "")
	registerPhase6Key(t, ctx, registry, cluster, "age1keep", true)
	registerPhase6Key(t, ctx, registry, cluster, "age1revoke", false)

	_, err := revoker.RevokeByFingerprint(ctx, RevokeOptions{
		Cluster: cluster, Fingerprint: "age1revoke",
	})
	require.NoError(t, err)
	keys, listErr := registry.ListKeys(ctx, cluster)
	require.NoError(t, listErr)
	for _, key := range keys {
		if key.Fingerprint == "age1revoke" {
			assert.Equal(t, KeyStatusRevoked, key.Status)
		}
	}
}
