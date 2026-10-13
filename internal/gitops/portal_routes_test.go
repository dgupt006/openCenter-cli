package gitops

import (
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	"github.com/stretchr/testify/require"
)

func TestHeadlampAndWeaveGitOpsOverlayRoutesUseClusterFQDN(t *testing.T) {
	cfg := newDefault("portal-routes")
	cfg.OpenCenter.Cluster.ClusterFQDN = "portal-routes.sjc3.k8s.opencenter.cloud"
	cfg.OpenCenter.Services["headlamp"].(*services.HeadlampConfig).Hostname = "dashboard.portal-routes.sjc3.k8s.opencenter.cloud"

	headlamp, err := headlampOverlayFilesRenderer(cfg)
	require.NoError(t, err)
	require.Contains(t, headlamp["headlamp-http-route.yaml"], "dashboard.portal-routes.sjc3.k8s.opencenter.cloud")
	require.Contains(t, headlamp["headlamp-http-route.yaml"], "sectionName: headlamp-https")
	require.Contains(t, headlamp["headlamp-oidc-ca-certificate.yaml"], "secretName: headlamp-oidc-ca")
	require.Contains(t, headlamp["headlamp-oidc-ca-certificate.yaml"], "name: rackspace-ca")

	weave, err := weaveGitOpsOverlayFilesRenderer(cfg)
	require.NoError(t, err)
	require.Contains(t, weave["weave-gitops-http-route.yaml"], "gitops.portal-routes.sjc3.k8s.opencenter.cloud")
	require.Contains(t, weave["weave-gitops-http-route.yaml"], "sectionName: gitops-https")
}
