package gitops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/secretartifacts"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

const etcdBackupCronJobTemplatePath = "templates/cluster-apps-base/services/etcd-backup/cronjob.yaml"

func TestBuiltInRenderCatalogAssociatesEtcdBackupDynamicPlanner(t *testing.T) {
	planner, ok := newBuiltInRenderCatalog().dynamicPlannerForDescriptor("service-etcd-backup")
	require.True(t, ok)
	require.NotNil(t, planner)
}

func TestEtcdBackupCronJobUsesGeneratedSecretKeys(t *testing.T) {
	data, err := Files.ReadFile(etcdBackupCronJobTemplatePath)
	require.NoError(t, err)
	content := string(data)
	for _, key := range []string{
		"ETCDCTL-API", "ETCDCTL-ENDPOINTS", "ETCDCTL-CACERT", "ETCDCTL-CERT", "ETCDCTL-KEY",
		"ACCESS-KEY", "SECRET-KEY", "S3-HOST", "S3-REGION", "S3-BUCKET-NAME",
	} {
		require.Contains(t, content, "key: "+key)
	}
	for _, key := range []string{
		"ETCDCTL_API", "ETCDCTL_ENDPOINTS", "ETCDCTL_CACERT", "ETCDCTL_CERT", "ETCDCTL_KEY",
		"ACCESS_KEY", "SECRET_KEY", "S3_HOST", "S3_REGION", "S3_BUCKET_NAME",
	} {
		require.NotContains(t, content, "key: "+key)
	}
	const image = "csengteam/etcd-backup@sha256:c1ba8c236d4b6ca45b61df9356b540abbb9ede20d8613b0012631b79123d84bc"
	require.Equal(t, 2, strings.Count(content, "image: "+image))
	require.NotContains(t, content, "csengteam/etcd-backup:v0.0.5")
	require.Contains(t, content, "mountPath: /backup.py")
	require.Contains(t, content, "subPath: backup.py")

	var manifest map[string]any
	require.NoError(t, yaml.Unmarshal(data, &manifest))
	require.Equal(t, "CronJob", manifest["kind"])
}

func TestEtcdBackupUploadScriptUsesConfiguredDestinationAndSigV4(t *testing.T) {
	data, err := Files.ReadFile("templates/cluster-apps-base/services/etcd-backup/backup.py")
	require.NoError(t, err)
	content := string(data)
	for _, variable := range []string{"ACCESS_KEY", "SECRET_KEY", "S3_HOST", "S3_REGION", "S3_BUCKET_NAME"} {
		require.Contains(t, content, `required_env("`+variable+`")`)
	}
	require.Contains(t, content, `Config(signature_version="s3v4")`)
	require.NotContains(t, content, `"etcd-backups"`)
	require.NotContains(t, content, "create_bucket")
	require.NotContains(t, content, "list_buckets")
}

func TestEtcdBackupFullAndSinglePlansHaveIdenticalOwnedOutputs(t *testing.T) {
	cfg := etcdBackupTestConfig(t, "etcd-planner-parity")

	full, artifacts, err := planClusterAppActionsWithArtifacts(cfg)
	require.NoError(t, err)
	single, _, err := planSingleServiceActionsWithArtifacts(cfg, "etcd-backup", false)
	require.NoError(t, err)

	fullOwned := ownedEtcdBackupActions(full)
	singleOwned := ownedEtcdBackupActions(single)
	require.Equal(t, ownedActionOutputs(singleOwned), ownedActionOutputs(fullOwned))
	before := actionContent(fullOwned, "services/etcd-backup/kustomization.yaml")
	require.Equal(t, before, actionContent(singleOwned, "services/etcd-backup/kustomization.yaml"))
	parsedBefore := parseEtcdBackupKustomization(t, before)
	require.Equal(t, []string{"cronjob.yaml"}, parsedBefore.Resources)
	require.Len(t, parsedBefore.ConfigMapGenerator, 1)
	require.Equal(t, "etcd-backup-upload-script", parsedBefore.ConfigMapGenerator[0].Name)
	require.Equal(t, []string{"backup.py"}, parsedBefore.ConfigMapGenerator[0].Files)

	var artifact secretartifacts.Artifact
	for _, candidate := range artifacts {
		if candidate.TargetService == "etcd-backup" {
			artifact = candidate
			break
		}
	}
	require.Equal(t, "services/etcd-backup/secret.yaml", artifact.Path)
	materializeSecretArtifact(t, cfg, artifact.Path)

	full, _, err = planClusterAppActionsWithArtifacts(cfg)
	require.NoError(t, err)
	single, _, err = planSingleServiceActionsWithArtifacts(cfg, "etcd-backup", false)
	require.NoError(t, err)
	fullOwned = ownedEtcdBackupActions(full)
	singleOwned = ownedEtcdBackupActions(single)
	require.Equal(t, ownedActionOutputs(singleOwned), ownedActionOutputs(fullOwned))
	after := actionContent(fullOwned, "services/etcd-backup/kustomization.yaml")
	require.Equal(t, after, actionContent(singleOwned, "services/etcd-backup/kustomization.yaml"))
	parsedAfter := parseEtcdBackupKustomization(t, after)
	require.Equal(t, []string{"cronjob.yaml", "secret.yaml"}, parsedAfter.Resources)
}

type renderedEtcdBackupKustomization struct {
	Resources          []string `yaml:"resources"`
	ConfigMapGenerator []struct {
		Name  string   `yaml:"name"`
		Files []string `yaml:"files"`
	} `yaml:"configMapGenerator"`
}

func parseEtcdBackupKustomization(t *testing.T, content string) renderedEtcdBackupKustomization {
	t.Helper()
	var parsed renderedEtcdBackupKustomization
	require.NoError(t, yaml.Unmarshal([]byte(content), &parsed))
	return parsed
}

func etcdBackupTestConfig(t *testing.T, name string) v2.Config {
	t.Helper()
	cfg := newDefault(name)
	repo, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	cfg.OpenCenter.GitOps.Repository.LocalDir = repo
	cfg.OpenCenter.Services["etcd-backup"] = &services.EtcdBackupConfig{
		BaseConfig:   services.BaseConfig{Enabled: true, Namespace: "kube-system"},
		S3Endpoint:   "https://s3.example/v1",
		S3BucketName: "etcd-backups",
		S3Region:     "RegionOne",
	}
	cfg.Secrets.EtcdBackup.AccessKeyID = "access-key"
	cfg.Secrets.EtcdBackup.SecretAccessKey = "secret-key"
	return cfg
}

func ownedEtcdBackupActions(actions []clusterAppAction) []clusterAppAction {
	result := make([]clusterAppAction, 0)
	for _, action := range actions {
		if action.Owner == "service-etcd-backup" || strings.HasPrefix(action.Output, "services/etcd-backup/") && action.Owner == "service-etcd-backup" {
			result = append(result, action)
		}
	}
	return result
}

func ownedActionOutputs(actions []clusterAppAction) []string {
	result := make([]string, 0, len(actions))
	for _, action := range actions {
		result = append(result, action.Output)
	}
	return result
}

func actionContent(actions []clusterAppAction, output string) string {
	for _, action := range actions {
		if action.Output == output {
			return action.Content
		}
	}
	return ""
}

// TestEtcdBackupWiredIntoFluxAggregator verifies etcd-backup is actually
// reconcilable by Flux: it must produce a top-level Flux Kustomization
// (services/fluxcd/etcd-backup.yaml) AND be listed in the services/fluxcd
// aggregator kustomization. Without both, Flux never sees the service.
func TestEtcdBackupWiredIntoFluxAggregator(t *testing.T) {
	cfg := etcdBackupTestConfig(t, "etcd-flux-wiring")
	require.NoError(t, RenderClusterApps(cfg))

	clusterRoot := filepath.Join(cfg.OpenCenter.GitOps.Repository.LocalDir, "applications", "overlays", cfg.ClusterName())

	// Top-level Flux Kustomization must exist and point at the etcd-backup overlay.
	fluxBridge, err := os.ReadFile(filepath.Join(clusterRoot, "services", "fluxcd", "etcd-backup.yaml"))
	require.NoError(t, err, "etcd-backup must produce a top-level Flux Kustomization")
	require.Contains(t, string(fluxBridge), "kind: Kustomization")
	require.Contains(t, string(fluxBridge), "name: etcd-backup")
	require.Contains(t, string(fluxBridge), "services/etcd-backup")

	// The aggregator must reference it, or Flux never reconciles it.
	aggregator, err := os.ReadFile(filepath.Join(clusterRoot, "services", "fluxcd", "kustomization.yaml"))
	require.NoError(t, err)
	require.Contains(t, string(aggregator), "./etcd-backup.yaml",
		"services/fluxcd aggregator must list etcd-backup so Flux reconciles it")
}

// TestEtcdBackupScopedRenderWiresAggregator verifies a scoped
// `service enable etcd-backup --render` also refreshes the aggregator to list
// etcd-backup (via the descriptor's aggregate_targets).
func TestEtcdBackupScopedRenderWiresAggregator(t *testing.T) {
	// Baseline full render WITHOUT etcd-backup to establish the tree + manifest.
	baselineCfg := newDefault("etcd-flux-scoped")
	repo, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	baselineCfg.OpenCenter.GitOps.Repository.LocalDir = repo
	require.NoError(t, RenderClusterApps(baselineCfg))

	clusterRoot := filepath.Join(repo, "applications", "overlays", baselineCfg.ClusterName())
	aggregator := filepath.Join(clusterRoot, "services", "fluxcd", "kustomization.yaml")
	before, err := os.ReadFile(aggregator)
	require.NoError(t, err)
	require.NotContains(t, string(before), "./etcd-backup.yaml")

	// Now enable etcd-backup on the same repo and render only that service.
	cfg := baselineCfg
	cfg.OpenCenter.Services["etcd-backup"] = &services.EtcdBackupConfig{
		BaseConfig:   services.BaseConfig{Enabled: true, Namespace: "kube-system"},
		S3Endpoint:   "https://s3.example/v1",
		S3BucketName: "etcd-backups",
		S3Region:     "RegionOne",
	}
	cfg.Secrets.EtcdBackup.AccessKeyID = "access-key"
	cfg.Secrets.EtcdBackup.SecretAccessKey = "secret-key"

	require.NoError(t, RenderSingleService(cfg, "etcd-backup", false))
	after, err := os.ReadFile(aggregator)
	require.NoError(t, err)
	require.Contains(t, string(after), "./etcd-backup.yaml",
		"scoped etcd-backup render must refresh the aggregator to list it")
}
