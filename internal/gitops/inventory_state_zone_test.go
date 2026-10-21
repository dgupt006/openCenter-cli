package gitops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestRenderInfrastructurePassesStateZoneInventoryDir guards lifecycle
// contract v2: the rendered main.tf must pass the cluster state-zone inventory
// directory to the Kubespray module via the inventory_output_dir variable, so
// `tofu apply` writes the inventory exactly once, outside the GitOps worktree.
func TestRenderInfrastructurePassesStateZoneInventoryDir(t *testing.T) {
	for _, provider := range []string{"openstack", "baremetal", "vmware"} {
		t.Run(provider, func(t *testing.T) {
			dst := t.TempDir()
			organization := "acme"
			cfg := mustNewGitOpsTestConfig("inv-zone", provider)
			cfg.OpenCenter.Cluster.ClusterName = "inv-zone"
			cfg.OpenCenter.Meta.Organization = organization
			cfg.OpenCenter.GitOps.Repository.LocalDir = dst

			t.Setenv("OPENCENTER_CLUSTER_STATE_DIR", filepath.Join(dst, "state-zone"))

			require.NoError(t, RenderInfrastructureCluster(cfg))

			mainTf := filepath.Join(dst, "infrastructure", "clusters", "inv-zone", "main.tf")
			data, err := os.ReadFile(mainTf) //nolint:gosec // test reads a file it just rendered
			require.NoError(t, err)
			content := string(data)

			wantDir := filepath.Join(dst, "state-zone", organization, "inv-zone", "inventory")
			require.Contains(t, content, `inventory_output_dir                    = var.inventory_output_dir`,
				"%s: module must forward the rendered inventory_output_dir variable", provider)

			variablesTf := filepath.Join(dst, "infrastructure", "clusters", "inv-zone", "variables.tf")
			varData, err := os.ReadFile(variablesTf) //nolint:gosec // test reads a file it just rendered
			require.NoError(t, err)
			require.Contains(t, string(varData), `variable "inventory_output_dir"`,
				"variables.tf must declare inventory_output_dir")

			// The rendered state-zone dir is exercised through the funcMap;
			// verify the helper directly against the same config.
			require.Equal(t, wantDir, inventoryStateDir(cfg))
		})
	}
}

// TestInventoryStateDirRequiresOrgAndName verifies the fallback contract: a
// config without organization or cluster name renders no state-zone directory,
// which keeps legacy-mode rendering on the module default (./inventory).
func TestInventoryStateDirRequiresOrgAndName(t *testing.T) {
	cfg := mustNewGitOpsTestConfig("inv-fallback", "openstack")
	cfg.OpenCenter.Meta.Organization = ""
	require.Equal(t, "", inventoryStateDir(cfg))

	cfg.OpenCenter.Meta.Organization = "acme"
	cfg.OpenCenter.Cluster.ClusterName = ""
	cfg.OpenCenter.Meta.Name = ""
	require.Equal(t, "", inventoryStateDir(cfg))
}

// TestRenderedMakefileExportsStateZoneInventory verifies the rendered Makefile
// points ANSIBLE_INVENTORY at the state zone when derivable, and keeps the
// worktree fallback otherwise.
func TestRenderedMakefileExportsStateZoneInventory(t *testing.T) {
	dst := t.TempDir()
	cfg := mustNewGitOpsTestConfig("inv-make", "openstack")
	cfg.OpenCenter.Cluster.ClusterName = "inv-make"
	cfg.OpenCenter.Meta.Organization = "acme"
	cfg.OpenCenter.GitOps.Repository.LocalDir = dst
	t.Setenv("OPENCENTER_CLUSTER_STATE_DIR", filepath.Join(dst, "state-zone"))

	require.NoError(t, RenderInfrastructureCluster(cfg))

	data, err := os.ReadFile(filepath.Join(dst, "infrastructure", "clusters", "inv-make", "Makefile")) //nolint:gosec // test reads a file it just rendered
	require.NoError(t, err)
	require.Contains(t, string(data),
		"export ANSIBLE_INVENTORY = "+filepath.Join(dst, "state-zone", "acme", "inv-make", "inventory", "inventory.yaml"))

	// No organization: legacy worktree fallback.
	dst2 := t.TempDir()
	cfg2 := mustNewGitOpsTestConfig("inv-make2", "openstack")
	cfg2.OpenCenter.Cluster.ClusterName = "inv-make2"
	cfg2.OpenCenter.Meta.Organization = ""
	cfg2.OpenCenter.GitOps.Repository.LocalDir = dst2
	require.NoError(t, RenderInfrastructureCluster(cfg2))

	data2, err := os.ReadFile(filepath.Join(dst2, "infrastructure", "clusters", "inv-make2", "Makefile")) //nolint:gosec // test reads a file it just rendered
	require.NoError(t, err)
	require.True(t, strings.Contains(string(data2),
		"export ANSIBLE_INVENTORY = "+filepath.Join(dst2, "infrastructure", "clusters", "inv-make2", "inventory", "inventory.yaml")),
		"legacy fallback must keep the worktree inventory path")
}
