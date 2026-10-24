package gitops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// renderOverlayValues returns the parsed kube-prometheus-stack Helm
// override-values.yaml for cfg.
func renderKubePrometheusStackValues(t *testing.T, cfg v2.Config) map[string]any {
	t.Helper()
	repo := t.TempDir()
	cfg.OpenCenter.GitOps.Repository.LocalDir = repo
	require.NoError(t, RenderClusterApps(cfg))
	path := filepath.Join(repo, "applications", "overlays", cfg.ClusterName(),
		"services", "kube-prometheus-stack", "helm-values", "override-values.yaml")
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(data, &doc), "rendered override-values.yaml must parse")
	return doc
}

// TestKubePrometheusStackAlertmanagerRouteHasDefaultReceiver guards the
// top-level route receiver. Alertmanager refuses to load a config whose
// route omits receiver, which takes down alerting for the whole cluster.
func TestKubePrometheusStackAlertmanagerRouteHasDefaultReceiver(t *testing.T) {
	cfg := newDefaultServiceConfig(t)
	doc := renderKubePrometheusStackValues(t, cfg)

	am, ok := doc["alertmanager"].(map[string]any)
	require.True(t, ok)
	config, ok := am["config"].(map[string]any)
	require.True(t, ok)
	route, ok := config["route"].(map[string]any)
	require.True(t, ok)
	require.NotEmpty(t, route["receiver"], "alertmanager route must declare a default receiver")

	declared := map[string]bool{}
	receivers, ok := config["receivers"].([]any)
	require.True(t, ok)
	for _, r := range receivers {
		m, ok := r.(map[string]any)
		require.True(t, ok)
		declared[m["name"].(string)] = true
	}

	subRoutes, ok := route["routes"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, subRoutes)
	for _, sub := range subRoutes {
		m, ok := sub.(map[string]any)
		require.True(t, ok)
		require.True(t, declared[m["receiver"].(string)],
			"route.routes references undeclared receiver %v", m["receiver"])
	}
}

// TestKubePrometheusStackRemoteWriteRequiresMimir guards the remote-write gate:
// a hardcoded Mimir target on a cluster without Mimir leaves Prometheus
// reporting a failing remote-write forever.
func TestKubePrometheusStackRemoteWriteRequiresMimir(t *testing.T) {
	t.Run("mimir disabled", func(t *testing.T) {
		cfg := newDefaultServiceConfig(t)
		cfg.OpenCenter.Services["mimir"] = &services.MimirConfig{BaseConfig: services.BaseConfig{Enabled: false}}
		doc := renderKubePrometheusStackValues(t, cfg)
		spec := doc["prometheus"].(map[string]any)["prometheusSpec"].(map[string]any)
		require.NotContains(t, spec, "remoteWrite")
		require.NotContains(t, spec, "enableRemoteWriteReceiver")
		// Cluster-wide monitor discovery is independent of Mimir and must stay.
		require.Equal(t, false, spec["serviceMonitorSelectorNilUsesHelmValues"])
		require.Equal(t, false, spec["podMonitorSelectorNilUsesHelmValues"])
		require.Equal(t, false, spec["ruleSelectorNilUsesHelmValues"])
	})

	t.Run("mimir enabled", func(t *testing.T) {
		cfg := newDefaultServiceConfig(t)
		cfg.OpenCenter.Services["mimir"] = &services.MimirConfig{BaseConfig: services.BaseConfig{Enabled: true}}
		doc := renderKubePrometheusStackValues(t, cfg)
		spec := doc["prometheus"].(map[string]any)["prometheusSpec"].(map[string]any)
		require.Contains(t, spec, "remoteWrite")
		require.Equal(t, true, spec["enableRemoteWriteReceiver"])
	})
}

// TestGrafanaOAuthSecretMatchesRealmClient guards the pairing between the
// realm's grafana-oauth client and the Secret Grafana mounts. Keycloak
// authenticates the token exchange against the secret of the client named in
// client_id, so reusing the opencenter client secret yields invalid_client.
func TestGrafanaOAuthSecretMatchesRealmClient(t *testing.T) {
	const (
		opencenterSecret = "opencenter-client-secret"
		grafanaSecret    = "grafana-client-secret"
	)
	cfg := newDefaultServiceConfig(t)
	cfg.OpenCenter.Identity.OIDC.Enabled = true
	cfg.Secrets.Keycloak.ClientSecret = opencenterSecret
	cfg.Secrets.Keycloak.GrafanaClientSecret = grafanaSecret

	repo := t.TempDir()
	cfg.OpenCenter.GitOps.Repository.LocalDir = repo
	require.NoError(t, RenderClusterApps(cfg))
	root := filepath.Join(repo, "applications", "overlays", cfg.ClusterName())

	oauthSecret := mustReadFile(t, filepath.Join(root,
		"services", "kube-prometheus-stack", "grafana-oauth-secret.yaml"))
	require.Contains(t, oauthSecret, "client_id: grafana-oauth")
	require.Contains(t, oauthSecret, `client_secret: "`+grafanaSecret+`"`)

	realm := mustReadFile(t, filepath.Join(root,
		"services", "keycloak", "20-keycloak", "opencenter-realm.yaml"))
	grafanaBlock := clientBlock(t, realm, "grafana-oauth")
	require.Contains(t, grafanaBlock, `secret: "`+grafanaSecret+`"`)
	require.NotContains(t, grafanaBlock, opencenterSecret)

	opencenterBlock := clientBlock(t, realm, "opencenter")
	require.Contains(t, opencenterBlock, `secret: "`+opencenterSecret+`"`)

	// The Envoy Gateway OIDC policies authenticate as the opencenter client.
	// They live in the descriptor-rendered oidc-security-policies overlay.
	gatewaySecret := mustReadFile(t, filepath.Join(root,
		"services", "oidc-security-policies", "gateway-oidc-secret.yaml"))
	require.Contains(t, gatewaySecret, `client-secret: "`+opencenterSecret+`"`)
}

// clientBlock returns the realm YAML belonging to one clientId, up to the next
// sibling client entry.
func clientBlock(t *testing.T, realm, clientID string) string {
	t.Helper()
	marker := "clientId: " + clientID + "\n"
	start := strings.Index(realm, marker)
	require.Positive(t, start, "realm must define client %s", clientID)
	rest := realm[start+len(marker):]
	if next := strings.Index(rest, "\n      - clientId:"); next >= 0 {
		rest = rest[:next]
	}
	return realm[start : start+len(marker)+len(rest)]
}

// TestGrafanaRoleAttributePathUsesEmittedClaims guards that Grafana's role
// mapping only reads claims the realm's protocol mappers actually produce.
func TestGrafanaRoleAttributePathUsesEmittedClaims(t *testing.T) {
	cfg := newDefaultServiceConfig(t)
	doc := renderKubePrometheusStackValues(t, cfg)

	grafana, ok := doc["grafana"].(map[string]any)
	require.True(t, ok)
	ini, ok := grafana["grafana.ini"].(map[string]any)
	require.True(t, ok)
	oauth, ok := ini["auth.generic_oauth"].(map[string]any)
	require.True(t, ok)
	path, ok := oauth["role_attribute_path"].(string)
	require.True(t, ok)

	// The realm emits `groups` (oidc-group-membership-mapper) and `roles`
	// (oidc-usermodel-client-role-mapper scoped to grafana-oauth). Anything else
	// silently resolves every user to the fallback role.
	for _, claim := range []string{"groups[*]", "roles[*]"} {
		require.Contains(t, path, claim)
	}
	for _, claim := range []string{"realm_access", "client_roles", "grafana-admins", "grafana-editors"} {
		require.NotContains(t, path, claim, "role_attribute_path references a claim the realm does not emit")
	}
	// Client role names must match the realm's grafana-oauth client roles.
	for _, role := range []string{"grafana-admin", "grafana-editor", "grafana-viewer"} {
		require.Contains(t, path, role)
	}
}

// TestGrafanaAdminPasswordSecretOnlyWhenConfigured guards against the overlay
// emitting an empty-password Secret that Flux would reconcile over the real
// one applied during bootstrap.
func TestGrafanaAdminPasswordSecretOnlyWhenConfigured(t *testing.T) {
	render := func(t *testing.T, password string) string {
		t.Helper()
		cfg := newDefaultServiceConfig(t)
		cfg.Secrets.Grafana.AdminPassword = password
		repo := t.TempDir()
		cfg.OpenCenter.GitOps.Repository.LocalDir = repo
		require.NoError(t, RenderClusterApps(cfg))
		return filepath.Join(repo, "applications", "overlays", cfg.ClusterName(),
			"services", "kube-prometheus-stack", "grafana-admin-password.yaml")
	}

	t.Run("password unset", func(t *testing.T) {
		path := render(t, "")
		_, err := os.Stat(path)
		require.True(t, os.IsNotExist(err), "must not emit grafana-admin-password.yaml without a password")
	})

	t.Run("password set", func(t *testing.T) {
		path := render(t, "s3cr3t")
		content := mustReadFile(t, path)
		require.Contains(t, content, `admin-password: "s3cr3t"`)
		require.Contains(t, content, `admin-user: "admin"`)
	})
}

// TestOIDCSecurityPoliciesWithheldForPlaceholderSecrets guards the
// descriptor-rendered oidc-security-policies overlay: while the Keycloak
// client secrets are still the CHANGEME placeholder, no SecurityPolicy and no
// gateway-oidc-secret may reach Git (ScanGitOpsSecrets rejects the stub, and
// Envoy Gateway would authenticate with a credential Keycloak always rejects).
func TestOIDCSecurityPoliciesWithheldForPlaceholderSecrets(t *testing.T) {
	oidcDir := func(t *testing.T) string {
		t.Helper()
		cfg := newDefaultServiceConfig(t)
		cfg.OpenCenter.Identity.OIDC.Enabled = true
		repo := t.TempDir()
		cfg.OpenCenter.GitOps.Repository.LocalDir = repo
		require.NoError(t, RenderClusterApps(cfg))
		return filepath.Join(repo, "applications", "overlays", cfg.ClusterName(),
			"services", "oidc-security-policies")
	}

	t.Run("placeholder secrets", func(t *testing.T) {
		dir := oidcDir(t)
		for _, name := range []string{
			"alertmanager-security-policy.yaml",
			"prometheus-security-policy.yaml",
			"gateway-oidc-secret.yaml",
		} {
			_, err := os.Stat(filepath.Join(dir, name))
			require.Truef(t, os.IsNotExist(err), "must not emit %s with placeholder secrets", name)
		}
	})

	t.Run("real secrets", func(t *testing.T) {
		cfg := newDefaultServiceConfig(t)
		cfg.OpenCenter.Identity.OIDC.Enabled = true
		cfg.Secrets.Keycloak.ClientSecret = "real-opencenter-secret"
		cfg.Secrets.Keycloak.GrafanaClientSecret = "real-grafana-secret"
		repo := t.TempDir()
		cfg.OpenCenter.GitOps.Repository.LocalDir = repo
		require.NoError(t, RenderClusterApps(cfg))
		dir := filepath.Join(repo, "applications", "overlays", cfg.ClusterName(),
			"services", "oidc-security-policies")
		for _, name := range []string{
			"kustomization.yaml",
			"alertmanager-security-policy.yaml",
			"prometheus-security-policy.yaml",
			"gateway-oidc-secret.yaml",
		} {
			require.FileExistsf(t, filepath.Join(dir, name), "must emit %s once secrets are real", name)
		}
	})
}
