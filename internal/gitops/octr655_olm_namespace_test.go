package gitops

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOCTR655OLMBaseKustomizationPreservesManifestNamespaces(t *testing.T) {
	cfg := newDefaultServiceConfig(t)

	actions, err := planClusterAppActions(cfg)
	require.NoError(t, err)

	var olmContent string
	for _, action := range actions {
		if action.Output == "services/fluxcd/olm.yaml" {
			olmContent = action.Content
			break
		}
	}
	require.NotEmpty(t, olmContent, "planner did not produce services/fluxcd/olm.yaml")
	require.Contains(t, olmContent, "path: applications/base/services/olm")
	require.NotContains(t, olmContent, "targetNamespace:")
	require.NotContains(t, olmContent, "healthChecks:")
	require.NotContains(t, olmContent, "kind: HelmRelease")
}
