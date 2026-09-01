package gitops

import (
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

// PlanClusterAppsPromotion renders the cluster applications into an isolated
// workspace and reports what promotion to the real overlay would do. It never
// mutates the configured GitOps repository.
func PlanClusterAppsPromotion(cfg v2.Config) (*PromoteResult, error) {
	return PlanClusterAppsPromotionWithOptions(cfg, PromoteOptions{DryRun: true})
}
