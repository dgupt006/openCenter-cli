package gitops

import (
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
