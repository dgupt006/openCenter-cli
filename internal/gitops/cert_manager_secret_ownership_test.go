// Copyright 2025.
// Licensed under the Apache License, Version 2.0.

package gitops

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/secretartifacts"
	"github.com/stretchr/testify/require"
)

// The credential Secret is `secrets sync`'s file: sync's copy is SOPS-encrypted
// and its hash is in the ownership ledger. The renderer emits the same path from
// a plaintext Go template, so without an adoption rule `cluster generate` after
// `secrets sync` would put the credential back in git readable and desynchronize
// the recorded hash.

func TestRenderCertManagerPreservesSyncedCredentialSecret(t *testing.T) {
	cfg, _ := dnsProviderFixture(t, "cloudflare")
	cfg.Secrets.CertManager.Cloudflare = map[string]v2.CertManagerCloudflareCredential{
		"cloudflare": {Enabled: true, APIToken: "cf-token"},
	}

	// The state a completed `secrets sync` leaves behind: the encrypted file at
	// the planner's path, plus the ledger record whose hash matches it. That
	// record is what marks the path as sync's, and the only thing distinguishing
	// it from an arbitrary file sitting in a generator-owned directory.
	secretPath := credentialSecretPathFor(t, cfg, "cloudflare", "cloudflare")
	require.NoError(t, os.MkdirAll(filepath.Dir(secretPath), 0o755))
	encrypted := "apiVersion: v1\nkind: Secret\nmetadata:\n  name: opencenter-cloudflare-credentials-secret-cloudflare\n" +
		"type: Opaque\nstringData:\n  api-token: ENC[AES256_GCM,data:ciphertext,iv:aaa,tag:bbb,type:str]\n" +
		"sops:\n  lastmodified: \"2026-10-27T00:00:00Z\"\n  mac: ENC[AES256_GCM,data:mm,iv:ccc,tag:ddd,type:str]\n"
	require.NoError(t, os.WriteFile(secretPath, []byte(encrypted), 0o600))
	writeSyncSecretLedgerForTest(t, cfg, filepath.Base(secretPath), encrypted)
	base := filepath.Dir(secretPath)

	require.NoError(t, RenderClusterApps(cfg))

	require.Equal(t, encrypted, mustReadFile(t, secretPath),
		"cluster generate must not overwrite a materialized credential Secret with plaintext")

	// The issuer and the kustomization still reference the file, so the render
	// stays valid with the encrypted copy in place.
	issuer := mustReadFile(t, filepath.Join(base, "letsencrypt-cloudflare-issuer.yaml"))
	require.Contains(t, issuer, `name: "opencenter-cloudflare-credentials-secret-cloudflare"`)
	kustomization := mustReadFile(t, filepath.Join(base, "kustomization.yaml"))
	require.Contains(t, kustomization, "./opencenter-cloudflare-credentials-secret-cloudflare.yaml")
}

func TestRenderCertManagerWritesCredentialSecretWhenAbsent(t *testing.T) {
	cfg, _ := dnsProviderFixture(t, "cloudflare")
	cfg.Secrets.CertManager.Cloudflare = map[string]v2.CertManagerCloudflareCredential{
		"cloudflare": {Enabled: true, APIToken: "cf-token"},
	}

	require.NoError(t, RenderClusterApps(cfg))

	// A repository that has never been synced still gets the file, so the
	// kustomization never points at a missing resource. `secrets sync` encrypts
	// it in place afterwards.
	secretPath := credentialSecretPathFor(t, cfg, "cloudflare", "cloudflare")
	content := mustReadFile(t, secretPath)
	require.Contains(t, content, "name: opencenter-cloudflare-credentials-secret-cloudflare")
	require.Contains(t, content, "api-token: cf-token")
}

// A credential path can also be in the generator's own ownership ledger, as it
// would be after a generate that ran before `secrets sync` encrypted the file.
// That makes the encrypted bytes look like user tampering with a tracked
// generated file, and it makes the dropped plan entry look stale and prunable.
// Neither may happen: the credential stays on disk, unmodified, with its
// generator record intact.
func TestRenderCertManagerPreservesLedgerTrackedCredentialSecret(t *testing.T) {
	cfg, _ := dnsProviderFixture(t, "cloudflare")
	cfg.Secrets.CertManager.Cloudflare = map[string]v2.CertManagerCloudflareCredential{
		"cloudflare": {Enabled: true, APIToken: "cf-token"},
	}

	secretPath := credentialSecretPathFor(t, cfg, "cloudflare", "cloudflare")
	require.NoError(t, os.MkdirAll(filepath.Dir(secretPath), 0o755))
	encrypted := "apiVersion: v1\nkind: Secret\nmetadata:\n  name: opencenter-cloudflare-credentials-secret-cloudflare\n" +
		"type: Opaque\nstringData:\n  api-token: ENC[AES256_GCM,data:ciphertext,iv:aaa,tag:bbb,type:str]\n" +
		"sops:\n  lastmodified: \"2026-10-27T00:00:00Z\"\n  mac: ENC[AES256_GCM,data:mm,iv:ccc,tag:ddd,type:str]\n"
	require.NoError(t, os.WriteFile(secretPath, []byte(encrypted), 0o600))

	// The generator's own ownership ledger also carries the path, as it would
	// after a generate that ran before sync encrypted the file.
	repoRoot := cfg.GitDir()
	manifestPath := filepath.Join(repoRoot, filepath.FromSlash(filepath.Join(ownershipClustersDir, cfg.ClusterName()+".json")))
	require.NoError(t, os.MkdirAll(filepath.Dir(manifestPath), 0o755))
	relative := repositoryOverlayPath(cfg.ClusterName(), "services/cert-manager/opencenter-cloudflare-credentials-secret-cloudflare.yaml")
	manifest := GeneratedManifest{Version: ownershipManifestVersion, Scope: "cluster", Identity: cfg.ClusterName(), FileRecords: map[string]GeneratedFileInfo{
		relative: {SHA256: hashBytes([]byte("stale plaintext")), Mode: 0o644},
	}}
	data, err := json.Marshal(manifest)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(manifestPath, data, 0o644))

	// sync's ledger record is what makes the path sync's rather than the
	// generator's. Its hash deliberately does NOT match the encrypted bytes on
	// disk: that is the drift case, where the file is still sync's credential
	// and must still not be replaced by the plaintext template.
	writeSyncSecretLedgerForTest(t, cfg, filepath.Base(secretPath), "hash of an earlier revision")

	require.NoError(t, RenderClusterApps(cfg))
	require.Equal(t, encrypted, mustReadFile(t, secretPath),
		"cluster generate must not overwrite a materialized credential Secret with plaintext")

	// Prune must not treat the dropped plan entry as a stale generated file:
	// deleting it would take a live credential out of the cluster.
	require.FileExists(t, secretPath)
	require.Contains(t, mustReadFile(t, manifestPath), filepath.ToSlash(relative),
		"the generator must not forget a credential path it merely declined to write")
}

// credentialSecretPathFor resolves where a rendered credential Secret lands,
// through the same overlay layout RenderClusterApps writes to.
func credentialSecretPathFor(t *testing.T, cfg v2.Config, provider, name string) string {
	t.Helper()
	filename, err := secretartifacts.CertManagerCredentialFilename(provider, name)
	require.NoError(t, err)
	return filepath.Join(cfg.GitDir(), "applications", "overlays", cfg.ClusterName(),
		"services", "cert-manager", filename)
}

// resolveRealPathForGitOpsTest removes the /var -> /private/var symlink macOS
// puts on t.TempDir, which the secret-artifact ledger refuses to walk.
func resolveRealPathForGitOpsTest(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	require.NoError(t, err)
	return resolved
}

// writeSyncSecretLedgerForTest records a credential Secret in the secret
// artifact ledger the way `secrets sync` does, so the generator can tell the
// path is sync's rather than a stray file in a generator-owned directory.
func writeSyncSecretLedgerForTest(t *testing.T, cfg v2.Config, filename, contents string) {
	t.Helper()
	overlayDir := filepath.Join(cfg.GitDir(), "applications", "overlays", cfg.ClusterName())
	require.NoError(t, secretartifacts.WriteOwnershipStateAtomic(overlayDir,
		secretartifacts.OwnershipState{Version: secretartifacts.OwnershipStateVersion, Artifacts: []secretartifacts.OwnershipArtifact{{
			Path:   "services/cert-manager/" + filename,
			Owners: []string{"cloudflare"},
			Hash:   secretartifacts.HashBytes([]byte(contents)),
		}}}))
}
