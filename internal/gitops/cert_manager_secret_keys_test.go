package gitops

import (
	"path/filepath"
	"strings"
	"testing"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// TestCertManagerIssuerSecretKeysResolve guards the contract between the
// rendered credential Secret and every ClusterIssuer that references it:
// cert-manager silently ignores a secretKeyRef whose key does not exist, so a
// renamed key breaks DNS-01 solving with no obvious error.
func TestCertManagerIssuerSecretKeysResolve(t *testing.T) {
	cfg := newDefaultServiceConfig(t)
	repo := t.TempDir()
	cfg.OpenCenter.GitOps.Repository.LocalDir = repo
	cfg.Secrets.CertManager.AWS = map[string]v2.CertManagerAWSCredential{
		"prod": {Enabled: true, AWSAccessKey: "AKIAPROD", AWSSecretAccessKey: "prodsecret", Region: "us-east-1", DNSZones: []string{"example.com"}},
	}
	cfg.Secrets.CertManager.Cloudflare = map[string]v2.CertManagerCloudflareCredential{
		"cf": {Enabled: true, APIToken: "cftoken", DNSZones: []string{"example.net"}},
	}
	require.NoError(t, RenderClusterApps(cfg))
	base := filepath.Join(repo, "applications", "overlays", cfg.ClusterName(), "services", "cert-manager")

	// Collect every Secret's key set by name.
	secretKeys := map[string]map[string]bool{}
	for _, file := range []string{
		"opencenter-aws-credentials-secret-prod.yaml",
		"opencenter-cloudflare-credentials-secret-cf.yaml",
	} {
		var secret struct {
			Metadata struct {
				Name string `yaml:"name"`
			} `yaml:"metadata"`
			StringData map[string]string `yaml:"stringData"`
		}
		require.NoError(t, yaml.Unmarshal([]byte(mustReadFile(t, filepath.Join(base, file))), &secret))
		require.NotEmpty(t, secret.Metadata.Name)
		keys := map[string]bool{}
		for k := range secret.StringData {
			keys[k] = true
		}
		secretKeys[secret.Metadata.Name] = keys
		require.NotEmpty(t, keys, "%s must carry stringData", file)
	}

	// Every secretKeyRef in every issuer must resolve against its Secret.
	issuer := mustReadFile(t, filepath.Join(base, "letsencrypt-prod-issuer.yaml"))
	var doc struct {
		Spec struct {
			ACME struct {
				Solvers []struct {
					DNS01 struct {
						Route53 struct {
							AccessKeyIDSecretRef struct {
								Name string `yaml:"name"`
								Key  string `yaml:"key"`
							} `yaml:"accessKeyIDSecretRef"`
							SecretAccessKeySecretRef struct {
								Name string `yaml:"name"`
								Key  string `yaml:"key"`
							} `yaml:"secretAccessKeySecretRef"`
						} `yaml:"route53"`
					} `yaml:"dns01"`
				} `yaml:"solvers"`
			} `yaml:"acme"`
		} `yaml:"spec"`
	}
	require.NoError(t, yaml.Unmarshal([]byte(issuer), &doc))
	require.Len(t, doc.Spec.ACME.Solvers, 1)
	route53 := doc.Spec.ACME.Solvers[0].DNS01.Route53
	for _, ref := range []struct{ name, key string }{
		{route53.AccessKeyIDSecretRef.Name, route53.AccessKeyIDSecretRef.Key},
		{route53.SecretAccessKeySecretRef.Name, route53.SecretAccessKeySecretRef.Key},
	} {
		require.NotEmpty(t, ref.name, "issuer must reference a Secret by name")
		keys, ok := secretKeys[ref.name]
		require.True(t, ok, "issuer references unknown Secret %q", ref.name)
		require.True(t, keys[ref.key], "Secret %q has no key %q referenced by the issuer", ref.name, ref.key)
	}

	// The pre-2026.10 key pair must stay gone: nothing reads it, and its
	// presence invites re-pointing an issuer at a key that no longer works.
	awsSecret := secretKeys["opencenter-aws-credentials-secret-prod"]
	require.Equal(t, map[string]bool{"aws-access-key": true, "aws-secret-access-key": true}, awsSecret)
	require.False(t, strings.Contains(
		mustReadFile(t, filepath.Join(base, "opencenter-aws-credentials-secret-prod.yaml")), "\n  access-key-id:"))
}
