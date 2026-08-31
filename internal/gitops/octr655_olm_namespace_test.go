package gitops

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOCTR655OLMBaseKustomizationPreservesManifestNamespaces(t *testing.T) {
	cfg := newDefaultServiceConfig(t)

	cfg.OpenCenter.GitOps.Repository.LocalDir = t.TempDir()
	require.NoError(t, RenderClusterApps(cfg))

	olmPath := filepath.Join(cfg.OpenCenter.GitOps.Repository.LocalDir, "applications", "overlays", cfg.ClusterName(), "services", "fluxcd", "olm.yaml")
	olmContent := mustReadFile(t, olmPath)
	require.NotEmpty(t, olmContent, "renderer did not produce %s", olmPath)
	require.Contains(t, olmContent, "name: olm-base")
	require.Contains(t, olmContent, "name: opencenter-olm-config")
	require.Contains(t, olmContent, "path: applications/overlays/"+cfg.ClusterName()+"/services/olm")
	require.NotContains(t, olmContent, "targetNamespace:")
	require.NotContains(t, olmContent, "healthChecks:")
	require.NotContains(t, olmContent, "kind: HelmRelease")
}
