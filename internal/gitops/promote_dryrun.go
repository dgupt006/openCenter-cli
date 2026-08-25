package gitops

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

// PlanClusterAppsPromotion renders the cluster applications into an isolated
// workspace and reports what promotion to the real overlay would do. It never
// mutates the configured GitOps repository.
func PlanClusterAppsPromotion(cfg v2.Config) (*PromoteResult, error) {
	clusterName := cfg.ClusterName()
	if clusterName == "" {
		return nil, fmt.Errorf("cluster name is empty")
	}
	if cfg.GitDir() == "" {
		return nil, fmt.Errorf("opencenter.gitops.repository.local_dir must be set")
	}

	manager := NewWorkspaceManager(os.TempDir())
	workspace, err := manager.CreateWorkspace(context.Background(), cfg)
	if err != nil {
		return nil, fmt.Errorf("creating dry-run workspace: %w", err)
	}
	defer manager.CleanupWorkspace(context.Background(), workspace)

	if err := RenderClusterAppsAtomic(cfg, workspace); err != nil {
		return nil, fmt.Errorf("rendering cluster apps: %w", err)
	}

	workspaceOverlayDir := filepath.Join(workspace.RootDir, "applications", "overlays", clusterName)
	targetOverlayDir := filepath.Join(cfg.GitDir(), "applications", "overlays", clusterName)
	result, err := promoteOverlay(workspaceOverlayDir, targetOverlayDir, clusterName, PromoteOptions{DryRun: true})
	if err != nil {
		return nil, err
	}
	return result, nil
}
