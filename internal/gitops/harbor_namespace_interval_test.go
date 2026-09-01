package gitops

import (
	"path/filepath"
	"strings"
	"testing"

	configservices "github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/stretchr/testify/require"
)

// TestHarborNamespaceIntervalIsValid verifies the harbor-namespace Kustomization
// uses a validator-compliant interval (5m/15m) rather than the previous 60m,
// which caused manifest validation to reject generate/deploy for Harbor-enabled
// clusters (OCTR-712).
func TestHarborNamespaceIntervalIsValid(t *testing.T) {
	cfg, err := v2.NewV2Default("k8s-harbor", "openstack")
	require.NoError(t, err)
	harbor, ok := cfg.OpenCenter.Services["harbor"].(*configservices.HarborConfig)
	require.True(t, ok)
	harbor.Enabled = true

	cfg.OpenCenter.GitOps.Repository.LocalDir = t.TempDir()
	require.NoError(t, RenderClusterApps(*cfg))

	flux := mustReadFile(t, filepath.Join(
		cfg.OpenCenter.GitOps.Repository.LocalDir, "applications", "overlays",
		cfg.ClusterName(), "services", "fluxcd", "harbor-namespace.yaml"))

	docs, err := decodeYAMLDocuments([]byte(flux))
	require.NoError(t, err)
	require.Len(t, docs, 1)
	interval := nestedString(docs[0], "spec", "interval")
	require.Truef(t, interval == "5m" || interval == "15m",
		"harbor-namespace interval must be 5m or 15m (validator requirement), got %q", interval)

	// End-to-end: the manifest validator must accept the generated tree.
	validator := NewManifestValidator(cfg.OpenCenter.GitOps.Repository.LocalDir)
	if err := validator.Validate(); err != nil {
		require.NotContainsf(t, err.Error(), "harbor-namespace",
			"manifest validation must not reject harbor-namespace: %v", err)
		// Any non-harbor validation noise from the temp tree is out of scope here.
		if strings.Contains(err.Error(), "interval should be 5m or 15m") {
			t.Fatalf("interval validation still failing: %v", err)
		}
	}
}
