package gitops

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

// renderMainTf renders the infrastructure cluster tree and returns main.tf.
func renderMainTf(t *testing.T, cfg v2.Config) string {
	t.Helper()

	manager := NewWorkspaceManager(t.TempDir())
	ctx := context.Background()
	workspace, err := manager.CreateWorkspace(ctx, cfg)
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	defer manager.CleanupWorkspace(ctx, workspace)

	if err := RenderInfrastructureClusterAtomic(cfg, workspace); err != nil {
		t.Fatalf("render infrastructure cluster: %v", err)
	}

	data, err := os.ReadFile(workspace.GetPath(filepath.Join("infrastructure", "clusters", cfg.ClusterName(), "main.tf")))
	if err != nil {
		t.Fatalf("read rendered main.tf: %v", err)
	}
	return string(data)
}

// kubesprayModuleBlock returns the rendered `module "kubespray-cluster"` block.
func kubesprayModuleBlock(t *testing.T, mainTf string) string {
	t.Helper()

	start := strings.Index(mainTf, `module "kubespray-cluster" {`)
	if start < 0 {
		t.Fatalf("rendered main.tf has no kubespray-cluster module block")
	}
	depth := 0
	for i := start; i < len(mainTf); i++ {
		switch mainTf[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return mainTf[start : i+1]
			}
		}
	}
	t.Fatalf("unterminated kubespray-cluster module block")
	return ""
}

// Additional Linux worker pools must reach the Kubespray inventory: the nodes
// are provisioned by openstack-nova, and only nodes passed in worker_nodes land
// in oc_worker_nodes -> kube_node. Regression for the pool-missing-from-
// inventory defect.
func TestRenderInfrastructureClusterAddsLinuxPoolsToKubesprayInventory(t *testing.T) {
	cfg := newDefault("pool-inventory")
	cfg.OpenCenter.Cluster.ClusterName = "pool-inventory"
	cfg.OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorker = []v2.WorkerPoolConfig{
		{Name: "batch", Count: 2, Flavor: "m5.large"},
		{Name: "gpu", Count: 1, Flavor: "gpu.large"},
	}

	block := kubesprayModuleBlock(t, renderMainTf(t, cfg))

	if !strings.Contains(block, "worker_nodes = concat(") {
		t.Fatalf("kubespray worker_nodes does not concat pool nodes:\n%s", block)
	}
	for _, want := range []string{
		"module.openstack-nova.worker_nodes",
		"module.openstack-nova.additional_worker_pools_nodes",
	} {
		if !strings.Contains(block, want) {
			t.Errorf("kubespray worker_nodes concat missing %q", want)
		}
	}
}

// Without pools the concat must still be well-formed HCL: flatten over an empty
// map yields the default worker pool unchanged.
func TestRenderInfrastructureClusterKubesprayWorkerNodesWithoutPools(t *testing.T) {
	cfg := newDefault("pool-inventory-none")
	cfg.OpenCenter.Cluster.ClusterName = "pool-inventory-none"
	cfg.OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorker = nil

	content := renderMainTf(t, cfg)
	block := kubesprayModuleBlock(t, content)

	if strings.Contains(block, "worker_nodes                            = module.openstack-nova.worker_nodes") {
		t.Error("kubespray worker_nodes still passes only the default worker pool")
	}
	if !strings.Contains(block, "worker_nodes = concat(") ||
		!strings.Contains(block, "additional_worker_pools_nodes") {
		t.Errorf("pool-less render lost the concat shape:\n%s", block)
	}
	if strings.Contains(content, "additional_server_pools_worker = local.additional_server_pools_worker") {
		t.Error("nova received an additional_server_pools_worker argument with no pools configured")
	}
	if strings.Count(content, "(") != strings.Count(content, ")") ||
		strings.Count(content, "[") != strings.Count(content, "]") {
		t.Error("rendered main.tf has unbalanced parentheses or brackets")
	}
}

// Windows pools keep their own concat and must not be duplicated into the
// Linux worker_nodes list.
func TestRenderInfrastructureClusterKeepsWindowsPoolsOutOfLinuxWorkerNodes(t *testing.T) {
	cfg := newDefault("pool-inventory-win")
	cfg.OpenCenter.Cluster.ClusterName = "pool-inventory-win"
	cfg.OpenCenter.Infrastructure.Compute.WorkerCountWindows = 1
	cfg.OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorker = []v2.WorkerPoolConfig{
		{Name: "batch", Count: 1, Flavor: "m5.large"},
	}
	cfg.OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorkerWindows = []v2.WindowsWorkerPoolConfig{
		{Name: "winbatch", Count: 1, Flavor: "m5.large", Image: "win2022"},
	}

	block := kubesprayModuleBlock(t, renderMainTf(t, cfg))

	workerArg := argumentLines(t, block, "worker_nodes")
	if strings.Contains(workerArg, "windows") {
		t.Errorf("Linux worker_nodes concat references Windows pool nodes:\n%s", workerArg)
	}
	if !strings.Contains(argumentLines(t, block, "windows_nodes"), "additional_worker_pools_windows_nodes") {
		t.Error("windows_nodes lost the additional Windows pool concat")
	}
}

// argumentLines returns the full `name = ...` assignment including continuations.
func argumentLines(t *testing.T, block, name string) string {
	t.Helper()

	lines := strings.Split(block, "\n")
	for i, line := range lines {
		if !strings.HasPrefix(strings.TrimSpace(line), name+" =") {
			continue
		}
		assignment := []string{line}
		for depth := bracketDepth(line); depth > 0 && i+1 < len(lines); i++ {
			assignment = append(assignment, lines[i+1])
			depth += bracketDepth(lines[i+1])
		}
		return strings.Join(assignment, "\n")
	}
	t.Fatalf("block has no %q argument:\n%s", name, block)
	return ""
}

// bracketDepth reports how many parens/brackets a line leaves open.
func bracketDepth(line string) int {
	return strings.Count(line, "(") + strings.Count(line, "[") -
		strings.Count(line, ")") - strings.Count(line, "]")
}
