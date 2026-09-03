package gitops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	descriptorcfg "github.com/opencenter-cloud/opencenter-cli/internal/services/descriptors"
	"github.com/stretchr/testify/require"
)

func TestBuiltInRenderCatalogValidates(t *testing.T) {
	catalog := newBuiltInRenderCatalog()
	require.NoError(t, catalog.Validate())

	gateway, ok := catalog.Lookup("gateway")
	require.True(t, ok)
	require.NotNil(t, gateway.OverlayFilesRenderer)
	require.Equal(t, "opencenter-gateway", gateway.SourceName)
	require.Equal(t, "applications/base/services/gateway", gateway.BasePath)

	loki, ok := catalog.Lookup("loki")
	require.True(t, ok)
	require.NotNil(t, loki.OverrideValuesRenderer)
	require.Equal(t, "opencenter-observability", loki.SourceName)
	require.Equal(t, "observability", loki.SourceGroup)
}

func TestBuildAutoServiceContextUsesCatalogRenderingMetadata(t *testing.T) {
	cfg := newAutoTestConfig("catalog-context")
	base := &services.BaseConfig{
		Enabled:   true,
		Namespace: "custom-namespace",
	}

	ctx := buildAutoServiceContext("gateway", base, cfg)

	// Namespace and supported source credentials remain declarative inputs.
	require.Equal(t, "custom-namespace", ctx.Namespace)
	// Rendering/topology metadata comes only from the built-in catalog.
	require.Equal(t, "opencenter-gateway", ctx.SourceName)
	require.Equal(t, "applications/base/services/gateway", ctx.BasePath)
	require.True(t, ctx.SingleStage)
	require.False(t, ctx.HasOverrideValues)
	require.Equal(t, []string{"namespace.yaml", "gateway-class.yaml", "gateway.yaml", "envoy-proxy-config.yaml"}, ctx.GeneratedResourceFiles)
	require.Equal(t, []string{"envoy-gateway-api-base"}, ctx.ExtraDependencies)
	require.NotNil(t, ctx.OverlayFilesRenderer)
	require.Empty(t, ctx.OverrideValuesRenderer)
}

func TestPlanAutoServiceActionsRejectsUnownedEnabledService(t *testing.T) {
	cfg := newAutoTestConfig("unowned-service")
	cfg.OpenCenter.Services["not-in-catalog"] = &services.DefaultServiceConfig{
		BaseConfig: services.BaseConfig{Enabled: true, Namespace: "unowned"},
	}
	registry, err := descriptorcfg.LoadEmbedded()
	require.NoError(t, err)

	_, err = planAutoServiceActions(cfg, registry)
	require.Error(t, err)
	require.Contains(t, err.Error(), `enabled service "not-in-catalog" has neither an explicit descriptor nor a built-in render catalog entry`)
}

func TestRenderCatalogRejectsDescriptorConflict(t *testing.T) {
	catalog := RenderCatalog{specs: []RenderSpec{{
		ServiceName:      "cert-manager",
		DefaultNamespace: "cert-manager",
		SourceName:       "opencenter-cert-manager",
		SourceGroup:      "cert-manager",
		BasePath:         "applications/base/services/cert-manager",
	}}}
	registry, err := descriptorcfg.LoadEmbedded()
	require.NoError(t, err)

	err = catalog.ValidateAgainstDescriptors(registry)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), `owned by both render catalog and descriptor`))
}

func TestBuiltInRenderCatalogAssociatesCertManagerDynamicPlanner(t *testing.T) {
	catalog := newBuiltInRenderCatalog()

	planner, ok := catalog.dynamicPlannerForDescriptor("service-cert-manager")
	require.True(t, ok)
	require.NotNil(t, planner)
}

func TestRenderCatalogRejectsDynamicPlannerWithoutExplicitDescriptor(t *testing.T) {
	catalog := RenderCatalog{dynamicPlanners: []catalogDynamicPlanner{{
		descriptorName: "missing-descriptor",
		planner:        planCertManagerDynamicActions,
	}}}
	registry, err := descriptorcfg.LoadEmbedded()
	require.NoError(t, err)

	err = catalog.ValidateAgainstDescriptors(registry)
	require.Error(t, err)
	require.Contains(t, err.Error(), `dynamic planner for descriptor "missing-descriptor" has no explicit descriptor`)
}

func TestRenderCatalogRejectsDynamicPlannerAutoOwnershipConflict(t *testing.T) {
	catalog := newBuiltInRenderCatalog()
	catalog.specs = append(catalog.specs, RenderSpec{
		ServiceName:      "cert-manager",
		DefaultNamespace: "cert-manager",
		SourceName:       "opencenter-cert-manager",
		SourceGroup:      "cert-manager",
		BasePath:         "applications/base/services/cert-manager",
	})
	registry, err := descriptorcfg.LoadEmbedded()
	require.NoError(t, err)

	err = catalog.ValidateAgainstDescriptors(registry)
	require.Error(t, err)
	require.Contains(t, err.Error(), `dynamic planner for descriptor "service-cert-manager" conflicts with auto-owned service "cert-manager"`)
}

func TestPlanClusterAppActionsIncludesCatalogDynamicActions(t *testing.T) {
	cfg := newDefault("catalog-full-plan")

	actions, _, err := planClusterAppActionsWithArtifacts(cfg)
	require.NoError(t, err)

	var found bool
	for _, action := range actions {
		if action.Output == "services/cert-manager/kustomization.yaml" {
			found = true
			require.Equal(t, "service-cert-manager", action.Owner)
			require.NotEmpty(t, action.Content)
		}
	}
	require.True(t, found, "full plan should include cert-manager dynamic kustomization")
}

func TestPlanSingleServiceActionsIncludesCatalogDynamicActions(t *testing.T) {
	cfg := newDefault("catalog-single-plan")

	actions, _, err := planSingleServiceActionsWithArtifacts(cfg, "cert-manager", false)
	require.NoError(t, err)

	var found bool
	for _, action := range actions {
		if action.Output == "services/cert-manager/kustomization.yaml" {
			found = true
			require.Equal(t, "service-cert-manager", action.Owner)
			require.NotEmpty(t, action.Content)
		}
	}
	require.True(t, found, "single-service plan should include cert-manager dynamic kustomization")
}

// TestRenderSingleServiceRefreshesAggregatorForAutoService verifies that a scoped
// enable --render of a catalog-owned (auto) NamespaceStage service also re-renders
// the top-level aggregator kustomization so the newly added namespace stage is
// listed. Without this, sealed-secrets/velero/openstack-ccm/openstack-csi scoped
// enables left services/fluxcd/kustomization.yaml stale (the operator had to
// hand-patch it).
func TestRenderSingleServiceRefreshesAggregatorForAutoService(t *testing.T) {
	cfg := newDefault("catalog-single-aggregator")
	cfg.OpenCenter.Cluster.ClusterName = "catalog-single-aggregator"
	cfg.OpenCenter.GitOps.Repository.LocalDir = t.TempDir()

	// Baseline full render WITHOUT sealed-secrets enabled: the aggregator must not
	// list a sealed-secrets namespace stage yet.
	if err := RenderClusterApps(cfg); err != nil {
		t.Fatalf("RenderClusterApps (baseline): %v", err)
	}
	clusterRoot := filepath.Join(cfg.OpenCenter.GitOps.Repository.LocalDir, "applications", "overlays", cfg.ClusterName())
	aggregator := filepath.Join(clusterRoot, "services", "fluxcd", "kustomization.yaml")
	baseline, err := os.ReadFile(aggregator)
	require.NoError(t, err)
	require.NotContains(t, string(baseline), "sealed-secrets-namespace",
		"baseline aggregator should not list sealed-secrets before it is enabled")

	// Enable sealed-secrets (a catalog-owned auto service with NamespaceStage: true)
	// and render ONLY that service, as `cluster service enable --render` does.
	cfg.OpenCenter.Services["sealed-secrets"] = &services.DefaultServiceConfig{
		BaseConfig: services.BaseConfig{Enabled: true, Namespace: "sealed-secrets"},
	}
	if err := RenderSingleService(cfg, "sealed-secrets", false); err != nil {
		t.Fatalf("RenderSingleService(sealed-secrets): %v", err)
	}

	// The scoped render must have refreshed the aggregator to include the new
	// top-level namespace-stage Kustomization.
	updated, err := os.ReadFile(aggregator)
	require.NoError(t, err)
	require.Contains(t, string(updated), "sealed-secrets-namespace",
		"scoped enable --render must refresh the aggregator to list the new NamespaceStage kustomization")
}

// TestSealedSecretsNamespaceStageWiring verifies the sealed-secrets fix
// (c6a602a): sealed-secrets uses a dedicated NamespaceStage so its -override can
// land its secretGenerator Secret into a namespace created independently of
// -base. The -override Kustomization must depend on sealed-secrets-namespace
// (not implicitly on -base), otherwise a second, namespace-creation deadlock
// occurs. Also verifies the namespace stage Kustomization is actually rendered.
func TestSealedSecretsNamespaceStageWiring(t *testing.T) {
	cfg := newDefault("sealed-secrets-nsstage")
	cfg.OpenCenter.Cluster.ClusterName = "sealed-secrets-nsstage"
	cfg.OpenCenter.GitOps.Repository.LocalDir = t.TempDir()
	cfg.OpenCenter.Services["sealed-secrets"] = &services.DefaultServiceConfig{
		BaseConfig: services.BaseConfig{Enabled: true, Namespace: "sealed-secrets"},
	}
	require.NoError(t, RenderClusterApps(cfg))

	clusterRoot := filepath.Join(cfg.OpenCenter.GitOps.Repository.LocalDir, "applications", "overlays", cfg.ClusterName())

	// The dedicated namespace-stage Kustomization must exist.
	nsStage := mustReadFile(t, filepath.Join(clusterRoot, "services", "fluxcd", "sealed-secrets-namespace.yaml"))
	require.NotEmpty(t, nsStage, "sealed-secrets NamespaceStage kustomization must be rendered")

	// The override must depend on the namespace stage.
	flux := mustReadFile(t, filepath.Join(clusterRoot, "services", "fluxcd", "sealed-secrets.yaml"))
	docs, err := decodeYAMLDocuments([]byte(flux))
	require.NoError(t, err)

	override := findFluxKustomization(t, docs, "sealed-secrets-override")
	require.True(t, hasFluxDependency(t, override, "sealed-secrets-namespace"),
		"sealed-secrets-override must depend on sealed-secrets-namespace so the secretGenerator Secret has a namespace to land in")
}
