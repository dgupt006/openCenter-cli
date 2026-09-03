package gitops

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/stretchr/testify/require"
)

// The longhorn overlay-files template reads
// (index .OpenCenter.Services "longhorn").Hostname. text/template resolves that
// field against the concrete type stored in the ServiceMap, so the default
// service map must wire longhorn as *services.LonghornConfig. It previously
// stored *services.DefaultServiceConfig, which has no Hostname field, and
// enabling longhorn failed template execution with:
//
//	can't evaluate field Hostname in type *services.DefaultServiceConfig
//
// ServiceMap.UnmarshalYAML types services from the config registry, which maps
// longhorn to LonghornConfig, so the default map disagreeing with the registry
// meant a loaded config and a freshly constructed one behaved differently.

func TestDefaultServiceMapWiresLonghornWithHostname(t *testing.T) {
	cfg, err := v2.NewV2Default("k8s-longhorn", "openstack")
	require.NoError(t, err)

	longhorn, ok := cfg.OpenCenter.Services["longhorn"].(*services.LonghornConfig)
	require.Truef(t, ok, "default service map must wire longhorn as *services.LonghornConfig, got %T. "+
		"The overlay template reads .Hostname, which only exists on LonghornConfig.",
		cfg.OpenCenter.Services["longhorn"])

	require.NotEmpty(t, longhorn.Hostname, "longhorn Hostname default should be set")
	require.Equal(t, "longhorn."+cfg.OpenCenter.Cluster.ClusterFQDN, longhorn.Hostname)
}

func TestLonghornOverlayRendersWhenEnabled(t *testing.T) {
	cfg := enableLonghorn(t, "")

	files, err := longhornOverlayFilesRenderer(cfg)
	require.NoError(t, err, "longhorn overlay renderer must not fail when longhorn is enabled")

	route, ok := files["longhorn-http-route.yaml"]
	require.True(t, ok, "expected longhorn-http-route.yaml")
	require.Contains(t, route, "kind: HTTPRoute")
	require.Contains(t, route, `"longhorn.`+cfg.OpenCenter.Cluster.ClusterFQDN+`"`)
}

func TestLonghornOverlayHonoursCustomHostname(t *testing.T) {
	cfg := enableLonghorn(t, "storage.example.com")

	files, err := longhornOverlayFilesRenderer(cfg)
	require.NoError(t, err)

	require.Contains(t, files["longhorn-http-route.yaml"], `"storage.example.com"`)
}

// TestLonghornPlansCleanly covers the full planning path, which is where the
// original failure surfaced (auto-render service longhorn: overlay-files
// renderer "longhorn": ...).
func TestLonghornPlansCleanly(t *testing.T) {
	cfg := enableLonghorn(t, "")

	actions, err := planClusterAppActions(cfg)
	require.NoError(t, err)

	var found bool
	for _, action := range actions {
		if strings.HasSuffix(action.Output, "longhorn-http-route.yaml") {
			found = true
			break
		}
	}
	require.True(t, found, "planned actions should include the longhorn HTTPRoute overlay file")
}

// TestLonghornOverrideDependsOnGatewayAPI verifies the longhorn-override
// Kustomization (which applies the HTTPRoute) waits for the Gateway API CRDs
// via envoy-gateway-api-base, and still orders after longhorn-base (OCTR-709).
func TestLonghornOverrideDependsOnGatewayAPI(t *testing.T) {
	cfg := enableLonghorn(t, "")
	cfg.OpenCenter.GitOps.Repository.LocalDir = t.TempDir()
	require.NoError(t, RenderClusterApps(cfg))

	flux := mustReadFile(t, filepath.Join(
		cfg.OpenCenter.GitOps.Repository.LocalDir, "applications", "overlays",
		cfg.ClusterName(), "services", "fluxcd", "longhorn.yaml"))
	docs, err := decodeYAMLDocuments([]byte(flux))
	require.NoError(t, err)

	override := findFluxKustomization(t, docs, "longhorn-override")
	require.True(t, hasFluxDependency(t, override, "envoy-gateway-api-base"),
		"longhorn-override must depend on envoy-gateway-api-base so the HTTPRoute applies after the Gateway API CRDs exist")
	require.True(t, hasFluxDependency(t, override, "longhorn-base"),
		"longhorn-override must still depend on longhorn-base for base->override ordering")
}

// TestLonghornOverrideDisablesDefaultStorageClass verifies the longhorn override
// sets persistence.defaultClass: false so Longhorn does not mark its own
// StorageClass as the cluster default. The chart defaults this to true, which
// silently hijacked the configured Cinder default and routed PVCs from other
// services to the wrong backend.
func TestLonghornOverrideDisablesDefaultStorageClass(t *testing.T) {
	cfg := enableLonghorn(t, "")
	cfg.OpenCenter.GitOps.Repository.LocalDir = t.TempDir()
	require.NoError(t, RenderClusterApps(cfg))

	override := mustReadFile(t, filepath.Join(
		cfg.OpenCenter.GitOps.Repository.LocalDir, "applications", "overlays",
		cfg.ClusterName(), "services", "longhorn", "helm-values", "override-values.yaml"))

	require.Contains(t, override, "persistence:", "longhorn override must set persistence values:\n%s", override)
	require.Contains(t, override, "defaultClass: false",
		"longhorn override must set persistence.defaultClass: false so it does not hijack the cluster default StorageClass:\n%s", override)
	require.NotContains(t, override, "defaultClass: true",
		"longhorn override must not leave defaultClass enabled:\n%s", override)
}

// enableLonghorn returns a default config with longhorn enabled, optionally
// overriding its hostname.
func enableLonghorn(t *testing.T, hostname string) v2.Config {
	t.Helper()

	cfg, err := v2.NewV2Default("k8s-longhorn", "openstack")
	require.NoError(t, err)

	longhorn, ok := cfg.OpenCenter.Services["longhorn"].(*services.LonghornConfig)
	require.True(t, ok)
	longhorn.Enabled = true
	if hostname != "" {
		longhorn.Hostname = hostname
	}

	return *cfg
}
