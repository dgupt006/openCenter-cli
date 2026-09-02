package gitops

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/stretchr/testify/require"
)

func TestWriteClusterAppActionsRejectsUnsafeOutputsBeforeWrites(t *testing.T) {
	root := t.TempDir()
	workspace := &GitOpsWorkspace{RootDir: root, TempDir: root}
	absoluteTarget := filepath.Join(t.TempDir(), "absolute.yaml")

	tests := []struct {
		name        string
		output      string
		outsidePath string
	}{
		{
			name:        "traversal",
			output:      "../escape.yaml",
			outsidePath: filepath.Join(filepath.Dir(root), "escape.yaml"),
		},
		{
			name:        "absolute",
			output:      absoluteTarget,
			outsidePath: absoluteTarget,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := writeClusterAppActions([]clusterAppAction{{Owner: "test", Output: tt.output, Content: "secret"}}, root, v2.Config{}, workspace)
			require.Error(t, err)
			require.NoFileExists(t, tt.outsidePath)
			require.Empty(t, directoryFiles(t, root))
		})
	}
}

func TestCertManagerCredentialNamesRejectTraversalBeforePlanning(t *testing.T) {
	tests := []struct {
		name string
		cfg  func(*v2.Config)
		want string
	}{
		{
			name: "AWS",
			cfg: func(cfg *v2.Config) {
				cfg.Secrets.CertManager.AWS = map[string]v2.CertManagerAWSCredential{
					"../escape": {Enabled: true, AWSAccessKey: "access", AWSSecretAccessKey: "secret"},
				}
			},
			want: "secrets.cert_manager.aws",
		},
		{
			name: "Cloudflare",
			cfg: func(cfg *v2.Config) {
				cfg.Secrets.CertManager.Cloudflare = map[string]v2.CertManagerCloudflareCredential{
					"../../escape": {Enabled: true, APIToken: "token"},
				}
			},
			want: "secrets.cert_manager.cloudflare",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := newDefault("unsafe-cert-credential")
			tt.cfg(&cfg)

			actions, err := planCertManagerDynamicActions(cfg, nil)
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.want)
			require.Empty(t, actions)
		})
	}
}

func TestEnabledManagedAlertProxyIsIncludedInFullPlanAndRender(t *testing.T) {
	cfg := newDefault("managed-alert-proxy")
	cfg.OpenCenter.GitOps.Repository.LocalDir = t.TempDir()
	cfg.OpenCenter.ManagedServices["alert-proxy"] = &services.AlertProxyConfig{
		BaseConfig:          services.BaseConfig{Enabled: true},
		HTTPRouteFQDN:       "alerts.example.test",
		AlertManagerBaseURL: "https://alertmanager.example.test",
	}

	actions, _, err := planClusterAppActionsWithArtifacts(cfg)
	require.NoError(t, err)
	var found bool
	for _, action := range actions {
		if action.Output == "managed-services/alert-proxy/kustomization.yaml" {
			found = true
			break
		}
	}
	require.True(t, found, "full plan should include enabled managed alert-proxy")

	require.NoError(t, RenderClusterApps(cfg))
	require.FileExists(t, filepath.Join(cfg.OpenCenter.GitOps.Repository.LocalDir, "applications", "overlays", cfg.ClusterName(), "managed-services", "alert-proxy", "kustomization.yaml"))
}

func directoryFiles(t *testing.T, root string) []string {
	t.Helper()
	entries, err := os.ReadDir(root)
	require.NoError(t, err)
	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		files = append(files, entry.Name())
	}
	return files
}
