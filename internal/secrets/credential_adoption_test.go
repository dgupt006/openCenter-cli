// Copyright 2025.
// Licensed under the Apache License, Version 2.0.

package secrets

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/secretartifacts"
)

// The generator renders the cert-manager credential Secret from a plaintext Go
// template at exactly the path `secrets sync` plans to own, so a repository
// generated before the credential ever reached the secret ledger carries a
// readable credential there. Sync's ownership guard refused to touch it, and
// `--force` did not help: the refusal left the exposed credential in the tree
// with no supported way to encrypt it. These tests pin the adoption contract:
// --force adopts (and thereby encrypts) a credential-shaped artifact, a plain
// sync still refuses, and a generic per-service secret.yaml is never adopted.

func TestSyncSecretsForceAdoptsPlaintextCredentialArtifact(t *testing.T) {
	requireSOPSAvailable(t)
	ctx := context.Background()
	env := newCredentialSyncEnv(t, "adopt-cred-cluster")
	env.configure(t, "cloudflare", func(cfg *v2.Config) {
		cfg.Secrets.CertManager.Cloudflare = map[string]v2.CertManagerCloudflareCredential{
			"cloudflare": {Enabled: true, APIToken: testCloudflareToken},
		}
	})

	// The state a pre-fix `cluster generate` leaves behind: the renderer's
	// plaintext credential Secret on disk with NO record in the secret ledger.
	secretPath := env.credentialSecretPath(t, "cloudflare", "cloudflare")
	require.NoError(t, os.MkdirAll(filepath.Dir(secretPath), 0o755))
	plaintext := "apiVersion: v1\nkind: Secret\nmetadata:\n  name: opencenter-cloudflare-credentials-secret-cloudflare\n" +
		"type: Opaque\nstringData:\n  api-token: " + testCloudflareToken + "\n"
	require.NoError(t, os.WriteFile(secretPath, []byte(plaintext), 0o644))

	// Without --force the ownership refusal must still fire.
	refused, err := env.manager.SyncSecrets(ctx, SyncOptions{Cluster: env.clusterName})
	require.NoError(t, err)
	require.Len(t, refused.Errors, 1)
	require.Contains(t, refused.Errors[0].Error.Error(), "refusing to adopt")
	require.Equal(t, secretPath, refused.Errors[0].FilePath)

	// With --force the artifact is adopted, encrypted, and recorded.
	result, err := env.manager.SyncSecrets(ctx, SyncOptions{Cluster: env.clusterName, Force: true})
	require.NoError(t, err)
	require.Emptyf(t, result.Errors, "force sync errors: %+v", result.Errors)
	require.Contains(t, result.Updated, secretPath)

	data := mustReadFileForSyncTest(t, secretPath)
	require.NotContainsf(t, data, testCloudflareToken,
		"adopted credential stayed plaintext:\n%s", data)
	require.Contains(t, data, "ENC[AES256_GCM,data:")

	manifest := env.decryptManifest(t, ctx, secretPath)
	require.Equal(t, "opencenter-cloudflare-credentials-secret-cloudflare",
		metadataOfManifest(t, manifest)["name"])
	require.Equal(t, testCloudflareToken, stringDataOfManifest(t, manifest)["api-token"])

	record, found := env.loadOwnershipRecords(t)[relativeCredentialPath(t, "cloudflare", "cloudflare")]
	require.True(t, found, "adopted credential must enter the ownership ledger")
	require.Equal(t, secretartifacts.HashBytes([]byte(data)), record.Hash)

	// A follow-up sync sees the adopted file as its own and leaves it alone.
	third, err := env.manager.SyncSecrets(ctx, SyncOptions{Cluster: env.clusterName})
	require.NoError(t, err)
	require.Emptyf(t, third.Errors, "post-adoption sync errors: %+v", third.Errors)
	require.Contains(t, third.Unchanged, secretPath)
}

func TestSyncSecretsForceStillRefusesUnownedGenericSecret(t *testing.T) {
	requireSOPSAvailable(t)
	ctx := context.Background()
	env := newCredentialSyncEnv(t, "adopt-generic-cluster")
	env.configure(t, "cloudflare", func(cfg *v2.Config) {
		cfg.Secrets.CertManager.Cloudflare = map[string]v2.CertManagerCloudflareCredential{
			"cloudflare": {Enabled: true, APIToken: testCloudflareToken},
		}
	})

	// A hand-authored file at the generic per-service secret.yaml path is NOT a
	// credential artifact the generator produces, so --force must not adopt it.
	genericPath := filepath.Join(env.overlayPath, "services", "headlamp", "secret.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(genericPath), 0o755))
	foreign := "apiVersion: v1\nkind: Secret\nmetadata:\n  name: hand-authored\nstringData:\n  token: keep-me\n"
	require.NoError(t, os.WriteFile(genericPath, []byte(foreign), 0o600))

	result, err := env.manager.SyncSecrets(ctx, SyncOptions{Cluster: env.clusterName, Force: true})
	require.NoError(t, err)
	var refusal *SyncError
	for i := range result.Errors {
		if result.Errors[i].FilePath == genericPath {
			refusal = &result.Errors[i]
		}
	}
	require.NotNilf(t, refusal, "unowned generic secret.yaml must not be adopted even with --force: %+v", result.Errors)
	require.Contains(t, refusal.Error.Error(), "refusing to adopt")
	require.Equal(t, foreign, mustReadFileForSyncTest(t, genericPath),
		"a refused artifact must stay byte-identical on disk")
}

func relativeCredentialPath(t *testing.T, provider, name string) string {
	t.Helper()
	relative, err := secretartifacts.CertManagerCredentialArtifactPath(provider, name)
	require.NoError(t, err)
	return relative
}
