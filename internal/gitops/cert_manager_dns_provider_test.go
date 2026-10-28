package gitops

import (
	"path/filepath"
	"testing"

	configservices "github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/stretchr/testify/require"
)

// dnsProviderFixture returns a config whose only cert-manager input is
// dns_provider plus a single token, the shape a user gets from
// `cluster set opencenter.services.cert-manager.dns_provider=<provider>`.
func dnsProviderFixture(t *testing.T, provider string) (v2.Config, string) {
	t.Helper()
	// t.TempDir is symlinked under /var/folders on macOS; the secret-artifact
	// ledger refuses a symlinked overlay root, so resolve the real path.
	dst := resolveRealPathForGitOpsTest(t, t.TempDir())
	cfg := newDefault("dns-provider-" + provider)
	cfg.OpenCenter.GitOps.Repository.LocalDir = dst
	cfg.OpenCenter.Cluster.BaseDomain = "example.com"
	cfg.OpenCenter.Cluster.ClusterFQDN = "dns-provider-" + provider + ".dfw3.example.com"

	certManager := cfg.OpenCenter.Services["cert-manager"].(*configservices.CertManagerConfig)
	certManager.DNSProvider = provider
	certManager.Email = "ops@example.com"
	return cfg, dst
}

// dns_provider: cloudflare with only a token configured must render the
// Cloudflare ClusterIssuer, its Secret, and make it the gateway default issuer.
func TestRenderCertManagerCloudflareFromDNSProviderOnly(t *testing.T) {
	cfg, dst := dnsProviderFixture(t, "cloudflare")
	cfg.Secrets.CertManager.Cloudflare = map[string]v2.CertManagerCloudflareCredential{
		"cloudflare": {Enabled: true, APIToken: "cf-token"},
	}

	require.NoError(t, RenderClusterApps(cfg))

	base := filepath.Join(dst, "applications", "overlays", cfg.ClusterName(), "services", "cert-manager")
	issuer := mustReadFile(t, filepath.Join(base, "letsencrypt-cloudflare-issuer.yaml"))
	require.Contains(t, issuer, "name: letsencrypt-cloudflare")
	require.Contains(t, issuer, "cloudflare:")
	require.Contains(t, issuer, "name: \"opencenter-cloudflare-credentials-secret-cloudflare\"")
	// The zone selector defaults to the delegated base domain.
	require.Contains(t, issuer, "- example.com")

	kustomization := mustReadFile(t, filepath.Join(base, "kustomization.yaml"))
	require.Contains(t, kustomization, "./opencenter-cloudflare-credentials-secret-cloudflare.yaml")
	require.Contains(t, kustomization, "./letsencrypt-cloudflare-issuer.yaml")

	gateway := mustReadFile(t, filepath.Join(dst, "applications", "overlays", cfg.ClusterName(), "services", "gateway", "gateway.yaml"))
	require.Contains(t, gateway, "cert-manager.io/cluster-issuer: letsencrypt-cloudflare")
}

// dns_provider: route53 with only the AWS application credentials configured
// must render the Route53 ClusterIssuer and make it the gateway default issuer.
func TestRenderCertManagerRoute53FromDNSProviderOnly(t *testing.T) {
	cfg, dst := dnsProviderFixture(t, "route53")
	cfg.Secrets.Global.AWS.Application = v2.AWSScopedSecrets{
		AccessKey:       "AKIAIOSFODNN7EXAMPLE",
		SecretAccessKey: "secret",
		Region:          "us-east-1",
	}

	require.NoError(t, RenderClusterApps(cfg))

	base := filepath.Join(dst, "applications", "overlays", cfg.ClusterName(), "services", "cert-manager")
	issuer := mustReadFile(t, filepath.Join(base, "letsencrypt-route53-issuer.yaml"))
	require.Contains(t, issuer, "name: letsencrypt-route53")
	require.Contains(t, issuer, "route53:")
	require.Contains(t, issuer, "region: us-east-1")

	gateway := mustReadFile(t, filepath.Join(dst, "applications", "overlays", cfg.ClusterName(), "services", "gateway", "gateway.yaml"))
	require.Contains(t, gateway, "cert-manager.io/cluster-issuer: letsencrypt-route53")
}

// Designate needs no service credential; its issuer must still be the default.
func TestRenderCertManagerDesignateIsDefaultIssuer(t *testing.T) {
	cfg, dst := dnsProviderFixture(t, "designate")
	cfg.OpenCenter.Infrastructure.Networking.UseDesignate = true

	require.NoError(t, RenderClusterApps(cfg))

	base := filepath.Join(dst, "applications", "overlays", cfg.ClusterName(), "services", "cert-manager")
	require.Contains(t, mustReadFile(t, filepath.Join(base, "letsencrypt-designate-issuer.yaml")), "webhook:")

	gateway := mustReadFile(t, filepath.Join(dst, "applications", "overlays", cfg.ClusterName(), "services", "gateway", "gateway.yaml"))
	require.Contains(t, gateway, "cert-manager.io/cluster-issuer: letsencrypt-designate")
}

// A credential-less provider renders no issuer and leaves the gateway on the
// self-signed rackspace-ca, the pre-existing behaviour.
func TestRenderCertManagerWithoutCredentialsKeepsRackspaceCA(t *testing.T) {
	cfg, dst := dnsProviderFixture(t, "cloudflare")

	require.NoError(t, RenderClusterApps(cfg))

	base := filepath.Join(dst, "applications", "overlays", cfg.ClusterName(), "services", "cert-manager")
	entries, err := filepath.Glob(filepath.Join(base, "letsencrypt-*"))
	require.NoError(t, err)
	require.Empty(t, entries)

	gateway := mustReadFile(t, filepath.Join(dst, "applications", "overlays", cfg.ClusterName(), "services", "gateway", "gateway.yaml"))
	require.Contains(t, gateway, "cert-manager.io/cluster-issuer: rackspace-ca")
	require.NotContains(t, gateway, "letsencrypt-")
}

// An explicit dns_zones on the credential is never overwritten by the default.
func TestRenderCertManagerKeepsExplicitDNSZones(t *testing.T) {
	cfg, dst := dnsProviderFixture(t, "cloudflare")
	cfg.Secrets.CertManager.Cloudflare = map[string]v2.CertManagerCloudflareCredential{
		"prod": {Enabled: true, APIToken: "cf-token", DNSZones: []string{"delegated.example.org"}},
	}

	require.NoError(t, RenderClusterApps(cfg))

	base := filepath.Join(dst, "applications", "overlays", cfg.ClusterName(), "services", "cert-manager")
	issuer := mustReadFile(t, filepath.Join(base, "letsencrypt-prod-issuer.yaml"))
	require.Contains(t, issuer, "- delegated.example.org")
	require.NotContains(t, issuer, "- example.com")
}
