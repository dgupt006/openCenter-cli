package gitops

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOCTR657KyvernoRenderUsesDirectSourcePaths(t *testing.T) {
	cfg := newDefaultServiceConfig(t)

	actions, err := planClusterAppActions(cfg)
	require.NoError(t, err)

	var kyvernoFluxContent string
	for _, action := range actions {
		require.NotEqual(t, "services/kyverno/kustomization.yaml", action.Output,
			"Kyverno must not create a customer-repository service kustomization")
		require.NotContains(t, action.Content, "../base/kyverno/default-ruleset",
			"Kyverno actions must not contain the legacy cross-repository relative resource")
		require.NotContains(t, action.Content, "opencenter-kyverno-ruleset",
			"Kyverno actions must not reference the legacy ruleset source")

		if action.Output == "services/fluxcd/kyverno.yaml" {
			kyvernoFluxContent = action.Content
		}
	}

	require.NotEmpty(t, kyvernoFluxContent, "planner did not produce services/fluxcd/kyverno.yaml")
	require.Contains(t, kyvernoFluxContent, "name: opencenter-kyverno")
	require.Contains(t, kyvernoFluxContent, "path: applications/base/services/kyverno/policy-engine")
	require.Contains(t, kyvernoFluxContent, "path: applications/base/services/kyverno/default-ruleset")
}
