/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package secrets

import (
	"context"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	"github.com/opencenter-cloud/opencenter-cli/internal/sops"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
)

// TestSyncSecretsEnsuresMissingSOPSConfig is the regression for repositories
// whose .sops.yaml was never created (gitops repo recreated after cluster
// init, or created outside it). Rotation/revocation re-encryption and path
// matching silently treat a missing config as "no rules", so the sync that
// encrypts into the repository must repair it first.
func TestSyncSecretsEnsuresMissingSOPSConfig(t *testing.T) {
	manager, tmpDir, cleanup := setupTestManager(t)
	defer cleanup()

	cluster := "repair-sops-config"
	repoDir := filepath.Join(tmpDir, "repo")
	keyPath := filepath.Join(tmpDir, ".config", "sops", "age", "repair-key.txt")
	require.NoError(t, os.MkdirAll(filepath.Dir(keyPath), 0o755))
	km := crypto.NewDefaultKeyManager(filepath.Dir(keyPath))
	keyPair, err := km.GenerateAgeKey()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(keyPath, []byte("# public key: "+keyPair.PublicKey+"\n"+keyPair.PrivateKey+"\n"), 0o600))

	configData := `schema_version: "2.0"
opencenter:
  cluster:
    cluster_name: ` + cluster + `
  gitops:
    git_dir: ` + repoDir + `
secrets:
  sops_age_key_file: ` + keyPath + `
  cert_manager:
    aws_access_key: AKIAIOSFODNN7EXAMPLE
`
	writeManagerTestConfig(t, cluster, configData)

	sopsConfigPath := filepath.Join(repoDir, sops.SOPSConfigFileName)
	require.NoFileExists(t, sopsConfigPath)

	// Dry-run must not touch the repository.
	_, err = manager.SyncSecrets(context.Background(), SyncOptions{Cluster: cluster, DryRun: true})
	require.NoError(t, err)
	require.NoFileExists(t, sopsConfigPath, "dry-run must not create the SOPS config")

	// The real sync creates it, derived from the cluster's Age key.
	_, err = manager.SyncSecrets(context.Background(), SyncOptions{Cluster: cluster, Services: []string{"cert-manager"}})
	require.NoError(t, err)

	data, err := os.ReadFile(sopsConfigPath)
	require.NoError(t, err, "sync must create the missing repository .sops.yaml")
	assert.Contains(t, string(data), keyPair.PublicKey)

	// A hand-edited config (extra rotation recipients) is never rewritten.
	handEdited := "creation_rules:\n  - path_regex: .*\n    age: " + keyPair.PublicKey + ",age1addedrecipientzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz\n"
	require.NoError(t, os.WriteFile(sopsConfigPath, []byte(handEdited), 0o644))
	_, err = manager.SyncSecrets(context.Background(), SyncOptions{Cluster: cluster, Services: []string{"cert-manager"}})
	require.NoError(t, err)
	after, err := os.ReadFile(sopsConfigPath)
	require.NoError(t, err)
	assert.Equal(t, handEdited, string(after), "sync must never overwrite an existing .sops.yaml")
}

// TestSyncSecretsResolvesSOPSConfigPathForSecureLayout pins the repository
// root derivation: the overlay lives at <repo>/applications/overlays/<cluster>
// and the config belongs at the repository root, matching
// paths.ClusterPaths.SOPSConfigPath.
func TestSyncSecretsResolvesSOPSConfigPathForSecureLayout(t *testing.T) {
	tmpDir := t.TempDir()
	resolver := paths.NewPathResolver(filepath.Join(tmpDir, "clusters"))
	require.NoError(t, resolver.CreateClusterDirectories(context.Background(), "demo", "acme"))
	clusterPaths, err := resolver.Resolve(context.Background(), "demo", "acme")
	require.NoError(t, err)

	overlayPath := filepath.Join(clusterPaths.GitOpsDir, "applications", "overlays", "demo")
	repoRoot := filepath.Dir(filepath.Dir(filepath.Dir(overlayPath)))
	assert.Equal(t, clusterPaths.GitOpsDir, repoRoot)
	assert.Equal(t, clusterPaths.SOPSConfigPath, filepath.Join(repoRoot, sops.SOPSConfigFileName))
}
