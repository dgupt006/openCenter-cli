package gitops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/secretartifacts"
	descriptorcfg "github.com/opencenter-cloud/opencenter-cli/internal/services/descriptors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractBaseConfig(t *testing.T) {
	t.Run("extracts from DefaultServiceConfig pointer", func(t *testing.T) {
		cfg := &services.DefaultServiceConfig{
			BaseConfig: services.BaseConfig{
				Enabled:   true,
				Namespace: "test-ns",
				Edition:   "enterprise",
			},
		}
		base := extractBaseConfig(cfg)
		require.NotNil(t, base)
		assert.Equal(t, "test-ns", base.Namespace)
		assert.Equal(t, "enterprise", base.Edition)
	})

	t.Run("returns nil for non-struct", func(t *testing.T) {
		base := extractBaseConfig("not a struct")
		assert.Nil(t, base)
	})

	t.Run("returns nil for nil pointer", func(t *testing.T) {
		var cfg *services.DefaultServiceConfig
		base := extractBaseConfig(cfg)
		assert.Nil(t, base)
	})
}

func TestBuildAutoServiceContext(t *testing.T) {
	base := &services.BaseConfig{
		Enabled:                true,
		Namespace:              "metallb-system",
		Edition:                "community",
		EnterpriseRegistry:     false,
		GeneratedResourceFiles: []string{"ipaddresspool.yaml", "l2advertisement.yaml"},
		ExtraDependencies:      []string{"some-dep"},
	}

	cfg := newAutoTestConfig("test-cluster")

	ctx := buildAutoServiceContext("metallb", base, cfg)

	assert.Equal(t, "metallb", ctx.ServiceName)
	assert.Equal(t, "metallb-system", ctx.Namespace)
	assert.Equal(t, "opencenter-metallb", ctx.SourceName)
	assert.Equal(t, "applications/base/services/metallb/community", ctx.BasePath)
	assert.Equal(t, "test-cluster", ctx.ClusterName)
	assert.Equal(t, []string{"ipaddresspool.yaml", "l2advertisement.yaml"}, ctx.GeneratedResourceFiles)
	assert.Equal(t, []string{"some-dep"}, ctx.ExtraDependencies)
	assert.False(t, ctx.EnterpriseRegistry)
}

func TestBuildAutoServiceContextSharedSource(t *testing.T) {
	base := &services.BaseConfig{
		Enabled:    true,
		Namespace:  "observability",
		SourceName: "opencenter-observability",
		Edition:    "enterprise",
	}

	cfg := newAutoTestConfig("obs-cluster")

	ctx := buildAutoServiceContext("loki", base, cfg)

	assert.Equal(t, "opencenter-observability", ctx.SourceName)
	assert.Equal(t, "applications/base/services/observability/loki/enterprise", ctx.BasePath)
}

func TestBuildAutoServiceContext_ReleaseTag(t *testing.T) {
	base := &services.BaseConfig{
		Enabled:   true,
		Namespace: "test-ns",
	}

	t.Run("uses release as tag when set", func(t *testing.T) {
		cfg := newAutoTestConfig("tag-cluster")
		cfg.OpenCenter.GitOps.BaseRepo.Release = "2026.01.2"

		ctx := buildAutoServiceContext("test-svc", base, cfg)

		assert.Equal(t, "2026.01.2", ctx.RepoTag)
		assert.Empty(t, ctx.RepoBranch)
	})

	t.Run("falls back to branch when release is empty", func(t *testing.T) {
		cfg := newAutoTestConfig("branch-cluster")
		cfg.OpenCenter.GitOps.BaseRepo.Release = ""
		cfg.OpenCenter.GitOps.BaseRepo.Branch = "develop"

		ctx := buildAutoServiceContext("test-svc", base, cfg)

		assert.Empty(t, ctx.RepoTag)
		assert.Equal(t, "develop", ctx.RepoBranch)
	})
}

func TestRenderAutoServiceActions_WithReleaseTag(t *testing.T) {
	ctx := autoServiceContext{
		ServiceName:       "sealed-secrets",
		Namespace:         "sealed-secrets",
		SourceName:        "opencenter-sealed-secrets",
		BasePath:          "applications/base/services/sealed-secrets",
		HasOverrideValues: true,
		ClusterName:       "k8s-prod",
		BaseRepoURL:       "ssh://git@github.com/opencenter-cloud/openCenter-gitops-base.git",
		RepoTag:           "2026.01.2",
		GitopsAuthMethod:  gitopsAuthMethodSSH,
		FluxInterval:      "15m",
	}

	actions, err := renderAutoServiceActions(ctx, newAutoTestConfig(ctx.ClusterName))
	require.NoError(t, err)

	// Source should use tag in the active SSH block
	assert.Contains(t, actions[0].Content, "# --- ssh auth (active) ---")
	assert.Contains(t, actions[0].Content, `tag: "2026.01.2"`)
	// Alternative token block should be commented out
	assert.Contains(t, actions[0].Content, "# --- token auth (alternative) ---")
	assert.Contains(t, actions[0].Content, "# url: https://github.com/opencenter-cloud/openCenter-gitops-base.git")
}

func TestRenderAutoServiceActions_TwoStage(t *testing.T) {
	ctx := autoServiceContext{
		ServiceName:       "sealed-secrets",
		Namespace:         "sealed-secrets",
		SourceName:        "opencenter-sealed-secrets",
		BasePath:          "applications/base/services/sealed-secrets",
		HasOverrideValues: true,
		ClusterName:       "k8s-sandbox",
		BaseRepoURL:       "https://github.com/rackerlabs/openCenter-gitops-base.git",
		RepoBranch:        "main",
		GitopsAuthMethod:  gitopsAuthMethodToken,
		FluxInterval:      "15m",
	}

	actions, err := renderAutoServiceActions(ctx, newAutoTestConfig(ctx.ClusterName))
	require.NoError(t, err)

	// Should produce: source, fluxcd, kustomization, override-values
	assert.Len(t, actions, 4)

	// Source
	assert.Equal(t, "services/sources/opencenter-sealed-secrets.yaml", actions[0].Output)
	assert.Contains(t, actions[0].Content, "name: opencenter-sealed-secrets")
	assert.Contains(t, actions[0].Content, "# --- token auth (active) ---")
	assert.Contains(t, actions[0].Content, "url: https://github.com/rackerlabs/openCenter-gitops-base.git")
	assert.Contains(t, actions[0].Content, `branch: "main"`)
	assert.Contains(t, actions[0].Content, "secretRef:")
	assert.Contains(t, actions[0].Content, "name: opencenter-base")
	// Alternative SSH block should be commented out
	assert.Contains(t, actions[0].Content, "# --- ssh auth (alternative) ---")
	assert.Contains(t, actions[0].Content, "# url: ssh://git@github.com/rackerlabs/openCenter-gitops-base.git")

	// FluxCD two-stage
	assert.Equal(t, "services/fluxcd/sealed-secrets.yaml", actions[1].Output)
	assert.Contains(t, actions[1].Content, "name: sealed-secrets-base")
	assert.Contains(t, actions[1].Content, "name: sealed-secrets-override")
	assert.Contains(t, actions[1].Content, "dependsOn:")
	assert.Contains(t, actions[1].Content, "name: sealed-secrets-base")
	assert.Contains(t, actions[1].Content, "path: applications/base/services/sealed-secrets")
	assert.Contains(t, actions[1].Content, "path: ./applications/overlays/k8s-sandbox/services/sealed-secrets")

	// Kustomization
	assert.Equal(t, "services/sealed-secrets/kustomization.yaml", actions[2].Output)
	assert.Contains(t, actions[2].Content, "namespace: sealed-secrets")
	assert.Contains(t, actions[2].Content, "sealed-secrets-values-override")

	// Override values
	assert.Equal(t, "services/sealed-secrets/helm-values/override-values.yaml", actions[3].Output)
	assert.Equal(t, "---\n...\n", actions[3].Content)
}

func TestRenderAutoServiceActions_SingleStage(t *testing.T) {
	ctx := autoServiceContext{
		ServiceName:            "gateway",
		Namespace:              "gateway",
		SourceName:             "opencenter-gateway",
		BasePath:               "applications/base/services/gateway",
		SingleStage:            true,
		HasOverrideValues:      false,
		ClusterName:            "k8s-dev",
		BaseRepoURL:            "ssh://git@github.com/rackerlabs/openCenter-gitops-base.git",
		RepoBranch:             "main",
		GitopsAuthMethod:       gitopsAuthMethodSSH,
		FluxInterval:           "5m",
		ExtraDependencies:      []string{"gateway-api-base"},
		GeneratedResourceFiles: []string{"namespace.yaml", "gateway.yaml"},
	}

	actions, err := renderAutoServiceActions(ctx, newAutoTestConfig(ctx.ClusterName))
	require.NoError(t, err)

	// source + fluxcd + kustomization (no override-values since HasOverrideValues=false)
	assert.Len(t, actions, 3)

	// Source with SSH secretRef (active)
	assert.Contains(t, actions[0].Content, "# --- ssh auth (active) ---")
	assert.Contains(t, actions[0].Content, "secretRef:")
	assert.Contains(t, actions[0].Content, "name: opencenter-base")
	// Alternative token block should be commented out
	assert.Contains(t, actions[0].Content, "# --- token auth (alternative) ---")
	assert.Contains(t, actions[0].Content, "# url: https://github.com/rackerlabs/openCenter-gitops-base.git")

	// FluxCD single-stage
	assert.Contains(t, actions[1].Content, "name: gateway")
	assert.NotContains(t, actions[1].Content, "gateway-base")
	assert.Contains(t, actions[1].Content, "name: gateway-api-base")
	assert.Contains(t, actions[1].Content, "interval: 5m")

	// Kustomization with custom resources, no secretGenerator
	assert.NotContains(t, actions[2].Content, "secretGenerator")
	assert.Contains(t, actions[2].Content, "- namespace.yaml")
	assert.Contains(t, actions[2].Content, "- gateway.yaml")
}

func TestRenderAutoServiceActions_SharedSource(t *testing.T) {
	ctx := autoServiceContext{
		ServiceName:       "loki",
		Namespace:         "observability",
		SourceName:        "opencenter-observability", // shared source
		BasePath:          "applications/base/services/observability/loki/enterprise",
		HasOverrideValues: true,
		ClusterName:       "k8s-sandbox",
		BaseRepoURL:       "https://github.com/rackerlabs/openCenter-gitops-base.git",
		RepoBranch:        "main",
		GitopsAuthMethod:  gitopsAuthMethodToken,
		FluxInterval:      "15m",
		ExtraDependencies: []string{"observability-namespace"},
	}

	actions, err := renderAutoServiceActions(ctx, newAutoTestConfig(ctx.ClusterName))
	require.NoError(t, err)

	// No source file (shared source owned by another service)
	// fluxcd + kustomization + override-values = 3
	assert.Len(t, actions, 3)
	assert.Equal(t, "services/fluxcd/loki.yaml", actions[0].Output)
	assert.Contains(t, actions[0].Content, "name: opencenter-observability")
	assert.Contains(t, actions[0].Content, "name: observability-namespace")
}

func TestRenderAutoServiceActions_EnterpriseRegistry(t *testing.T) {
	ctx := autoServiceContext{
		ServiceName:            "headlamp",
		Namespace:              "headlamp",
		SourceName:             "opencenter-headlamp",
		BasePath:               "applications/base/services/headlamp/enterprise",
		HasOverrideValues:      true,
		EnterpriseRegistry:     true,
		GeneratedResourceFiles: []string{"httproute.yaml"},
		ClusterName:            "k8s-sandbox",
		BaseRepoURL:            "https://github.com/rackerlabs/openCenter-gitops-base.git",
		RepoBranch:             "main",
		GitopsAuthMethod:       gitopsAuthMethodToken,
		FluxInterval:           "15m",
	}

	actions, err := renderAutoServiceActions(ctx, newAutoTestConfig(ctx.ClusterName))
	require.NoError(t, err)

	// Find kustomization action
	var kustContent string
	for _, a := range actions {
		if strings.HasSuffix(a.Output, "kustomization.yaml") {
			kustContent = a.Content
			break
		}
	}
	require.NotEmpty(t, kustContent)
	assert.Contains(t, kustContent, "- httproute.yaml")
	assert.Contains(t, kustContent, `"../global/rackspace-registry/"`)
}

func TestRenderAutoServiceActions_BaseOnly(t *testing.T) {
	ctx := autoServiceContext{
		ServiceName:       "external-snapshotter",
		Namespace:         "external-snapshotter",
		SourceName:        "opencenter-external-snapshotter",
		BasePath:          "applications/base/services/external-snapshotter",
		BaseOnly:          true,
		HasOverrideValues: true, // should be ignored when BaseOnly
		ClusterName:       "k8s-sandbox",
		BaseRepoURL:       "https://github.com/rackerlabs/openCenter-gitops-base.git",
		RepoBranch:        "main",
		GitopsAuthMethod:  gitopsAuthMethodToken,
		FluxInterval:      "15m",
	}

	actions, err := renderAutoServiceActions(ctx, newAutoTestConfig(ctx.ClusterName))
	require.NoError(t, err)

	// BaseOnly: source + fluxcd only (no kustomization, no override-values)
	assert.Len(t, actions, 2)

	// Source
	assert.Equal(t, "services/sources/opencenter-external-snapshotter.yaml", actions[0].Output)

	// FluxCD — base only, no override stage
	assert.Equal(t, "services/fluxcd/external-snapshotter.yaml", actions[1].Output)
	assert.Contains(t, actions[1].Content, "name: external-snapshotter-base")
	assert.NotContains(t, actions[1].Content, "override")
	assert.NotContains(t, actions[1].Content, "sops")
}

func TestRenderAutoServiceActions_OverrideDependsOn(t *testing.T) {
	ctx := autoServiceContext{
		ServiceName:       "weave-gitops",
		Namespace:         "flux-system",
		SourceName:        "opencenter-weave-gitops",
		BasePath:          "applications/base/services/weave-gitops",
		HasOverrideValues: true,
		OverrideDependsOn: []string{"sources", "envoy-gateway-api-base"},
		ClusterName:       "k8s-dev",
		BaseRepoURL:       "ssh://git@github.com/rackerlabs/openCenter-gitops-base.git",
		RepoBranch:        "main",
		GitopsAuthMethod:  gitopsAuthMethodSSH,
		FluxInterval:      "5m",
	}

	actions, err := renderAutoServiceActions(ctx, newAutoTestConfig(ctx.ClusterName))
	require.NoError(t, err)

	// Find fluxcd action
	var fluxContent string
	for _, a := range actions {
		if a.Output == "services/fluxcd/weave-gitops.yaml" {
			fluxContent = a.Content
			break
		}
	}
	require.NotEmpty(t, fluxContent)

	// Override should depend on custom list, not weave-gitops-base
	assert.Contains(t, fluxContent, "name: sources")
	assert.Contains(t, fluxContent, "name: envoy-gateway-api-base")
	// Should NOT have the default dependsOn
	// The override section should not reference weave-gitops-base as dependsOn
	parts := strings.SplitN(fluxContent, "name: weave-gitops-override", 2)
	require.Len(t, parts, 2)
	assert.NotContains(t, parts[1], "name: weave-gitops-base")
}

func TestRenderAutoServiceActions_OverrideValues(t *testing.T) {
	ctx := autoServiceContext{
		ServiceName:       "sealed-secrets",
		Namespace:         "sealed-secrets",
		SourceName:        "opencenter-sealed-secrets",
		BasePath:          "applications/base/services/sealed-secrets",
		HasOverrideValues: true,
		OverrideValues:    "keyrenewperiod: \"0\"\n",
		ClusterName:       "k8s-sandbox",
		BaseRepoURL:       "https://github.com/rackerlabs/openCenter-gitops-base.git",
		RepoBranch:        "main",
		GitopsAuthMethod:  gitopsAuthMethodToken,
		FluxInterval:      "15m",
	}

	actions, err := renderAutoServiceActions(ctx, newAutoTestConfig(ctx.ClusterName))
	require.NoError(t, err)

	// Find override-values action
	var overrideContent string
	for _, a := range actions {
		if strings.HasSuffix(a.Output, "override-values.yaml") {
			overrideContent = a.Content
			break
		}
	}
	assert.Equal(t, "keyrenewperiod: \"0\"\n", overrideContent)
}

func TestHasExplicitDescriptor(t *testing.T) {
	registry, err := descriptorcfg.LoadEmbedded()
	require.NoError(t, err)

	// cert-manager has an explicit descriptor
	assert.True(t, hasExplicitDescriptor(registry, "cert-manager"))

	// a made-up service does not
	assert.False(t, hasExplicitDescriptor(registry, "nonexistent-service"))
}

func TestPlanAutoServiceActions_SkipsDescriptorServices(t *testing.T) {
	cfg := newAutoTestConfig("test-cluster")
	// cert-manager has an explicit descriptor, should be skipped
	cfg.OpenCenter.Services["cert-manager"] = &services.CertManagerConfig{
		BaseConfig: services.BaseConfig{Enabled: true, Namespace: "cert-manager"},
	}
	// Add a service without a descriptor
	cfg.OpenCenter.Services["my-new-service"] = &services.DefaultServiceConfig{
		BaseConfig: services.BaseConfig{
			Enabled:   true,
			Namespace: "my-ns",
		},
	}

	registry, err := descriptorcfg.LoadEmbedded()
	require.NoError(t, err)

	actions, err := planAutoServiceActions(cfg, registry)
	require.NoError(t, err)

	// Should only have actions for my-new-service, not cert-manager
	for _, a := range actions {
		assert.Contains(t, a.Owner, "my-new-service")
		assert.NotContains(t, a.Owner, "cert-manager")
	}
}

// newAutoTestConfig creates a minimal v2.Config for testing auto-descriptors.
func newAutoTestConfig(clusterName string) v2.Config {
	return v2.Config{
		OpenCenter: v2.OpenCenterConfig{
			Cluster: v2.ClusterConfig{
				ClusterName: clusterName,
			},
			GitOps: v2.GitOpsConfig{
				ResolvedAuthMethod: gitopsAuthMethodToken,
				Repository: v2.GitOpsRepository{
					Branch: "main",
				},
				BaseRepo: v2.GitOpsBaseRepo{
					URL:    "https://github.com/rackerlabs/openCenter-gitops-base.git",
					Branch: "main",
				},
				Flux: v2.GitOpsFluxConfig{
					Interval: "15m",
				},
			},
			Services: v2.ServiceMap{},
		},
	}
}

func TestBuildAutoServiceContextPreservesServiceSourceOverride(t *testing.T) {
	cfg := newAutoTestConfig("source-override")
	cfg.OpenCenter.GitOps.ResolvedAuthMethod = gitopsAuthMethodSSH
	cfg.OpenCenter.GitOps.BaseRepo.Release = "2026.01"
	base := &services.BaseConfig{
		Enabled: true,
		Source: services.ServiceSource{
			Repo:    "https://gitlab.example.com/team/custom-service.git",
			Branch:  "develop",
			Release: "",
		},
	}

	ctx := buildAutoServiceContext("custom-service", base, cfg)
	assert.Equal(t, "https://gitlab.example.com/team/custom-service.git", ctx.BaseRepoURL)
	assert.Equal(t, "develop", ctx.RepoBranch)
	assert.Empty(t, ctx.RepoTag)
	assert.Equal(t, gitopsAuthMethodSSH, ctx.GitopsAuthMethod)

	actions, err := renderAutoServiceActions(ctx, cfg)
	require.NoError(t, err)
	assert.Contains(t, actions[0].Content, "# --- ssh auth (active) ---")
	assert.Contains(t, actions[0].Content, "url: ssh://git@gitlab.example.com/team/custom-service.git")
	assert.Contains(t, actions[0].Content, "# url: https://gitlab.example.com/team/custom-service.git")
	assert.Contains(t, actions[0].Content, `branch: "develop"`)
}

func TestBuildAutoServiceContextConditionalSecretArtifact(t *testing.T) {
	base := &services.BaseConfig{Enabled: true, Namespace: "observability"}
	cfg := newAutoTestConfig("secret-artifact-cluster")
	repo, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	cfg.OpenCenter.GitOps.Repository.LocalDir = repo

	cfg.Secrets.Grafana = v2.GrafanaSecrets{}
	artifacts, err := secretartifacts.Plan(&cfg)
	require.NoError(t, err)
	ctx := buildAutoServiceContextWithArtifacts("kube-prometheus-stack", base, cfg, artifacts)
	assert.NotContains(t, ctx.GeneratedResourceFiles, "secret.yaml")

	cfg.Secrets.Grafana.AdminPassword = "grafana-password"
	artifacts, err = secretartifacts.Plan(&cfg)
	require.NoError(t, err)
	materializeSecretArtifact(t, cfg, artifacts[0].Path)
	ctx = buildAutoServiceContextWithArtifacts("kube-prometheus-stack", base, cfg, artifacts)
	assert.Contains(t, ctx.GeneratedResourceFiles, "secret.yaml")

	cfg.Secrets.Grafana = v2.GrafanaSecrets{}
	cfg.Secrets.ServiceSecrets = map[string]any{"custom_service": map[string]any{"token": "value"}}
	artifacts, err = secretartifacts.Plan(&cfg)
	require.NoError(t, err)
	materializeSecretArtifact(t, cfg, artifacts[0].Path)
	ctx = buildAutoServiceContextWithArtifacts("custom-service", base, cfg, artifacts)
	assert.Contains(t, ctx.GeneratedResourceFiles, "secret.yaml")
}

func materializeSecretArtifact(t *testing.T, cfg v2.Config, relativePath string) {
	t.Helper()
	materializeSecretArtifacts(t, cfg, []secretartifacts.Artifact{{Path: relativePath}})
}

func materializeSecretArtifacts(t *testing.T, cfg v2.Config, artifacts []secretartifacts.Artifact) {
	t.Helper()
	overlay := filepath.Join(cfg.GitDir(), "applications", "overlays", cfg.ClusterName())
	ownershipArtifacts := make([]secretartifacts.OwnershipArtifact, 0, len(artifacts))
	for _, artifact := range artifacts {
		path := filepath.Join(overlay, filepath.FromSlash(artifact.Path))
		data := []byte("materialized secret")
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, data, 0o600))
		ownershipArtifacts = append(ownershipArtifacts, secretartifacts.OwnershipArtifact{
			Path: artifact.Path,
			Hash: secretartifacts.HashBytes(data),
		})
	}
	require.NoError(t, secretartifacts.WriteOwnershipStateAtomic(overlay, secretartifacts.OwnershipState{
		Version:   secretartifacts.OwnershipStateVersion,
		Artifacts: ownershipArtifacts,
	}))
}

func TestAutoServiceSecretMembershipAfterSync(t *testing.T) {
	cfg := newAutoTestConfig("auto-secret-membership")
	repo, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	cfg.OpenCenter.GitOps.Repository.LocalDir = repo
	cfg.OpenCenter.Services["custom-service"] = &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{Enabled: true, Namespace: "custom"}}
	base := extractBaseConfig(cfg.OpenCenter.Services["custom-service"])

	before, err := renderAutoServiceActions(buildAutoServiceContextWithArtifacts("custom-service", base, cfg, nil), cfg)
	require.NoError(t, err)
	assert.NotContains(t, kustomizationContent(before), "secret.yaml")

	cfg.Secrets.ServiceSecrets = map[string]any{"custom_service": map[string]any{"token": "value"}}
	artifacts, err := secretartifacts.Plan(&cfg)
	require.NoError(t, err)
	require.Len(t, artifacts, 1)
	materializeSecretArtifact(t, cfg, artifacts[0].Path)

	after, err := renderAutoServiceActions(buildAutoServiceContextWithArtifacts("custom-service", base, cfg, artifacts), cfg)
	require.NoError(t, err)
	assert.Contains(t, kustomizationContent(after), "secret.yaml")
}

func TestValidateMaterializedSecretMembershipRejectsExplicitOmission(t *testing.T) {
	cfg := newAutoTestConfig("explicit-secret-omission")
	repo, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	cfg.OpenCenter.GitOps.Repository.LocalDir = repo
	cfg.OpenCenter.Services["custom-service"] = &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{Enabled: true, Namespace: "custom"}}
	cfg.Secrets.ServiceSecrets = map[string]any{"custom_service": map[string]any{"token": "value"}}
	artifacts, err := secretartifacts.Plan(&cfg)
	require.NoError(t, err)
	materializeSecretArtifact(t, cfg, artifacts[0].Path)

	rendered := filepath.Join(repo, "rendered")
	output := "services/custom-service/kustomization.yaml"
	require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(rendered, output)), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(rendered, output), []byte("apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\nresources: []\n"), 0o644))
	err = validateMaterializedSecretMembership(cfg, []clusterAppAction{{Output: output, Content: "explicit"}}, artifacts, rendered)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not include secret.yaml")
}

func TestValidateMaterializedSecretMembershipAcceptsAutoContent(t *testing.T) {
	cfg := newAutoTestConfig("auto-content-secret")
	repo, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	cfg.OpenCenter.GitOps.Repository.LocalDir = repo
	cfg.OpenCenter.Services["custom-service"] = &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{Enabled: true, Namespace: "custom"}}
	cfg.Secrets.ServiceSecrets = map[string]any{"custom_service": map[string]any{"token": "value"}}
	artifacts, err := secretartifacts.Plan(&cfg)
	require.NoError(t, err)
	materializeSecretArtifact(t, cfg, artifacts[0].Path)

	rendered := filepath.Join(repo, "rendered")
	output := "services/custom-service/kustomization.yaml"
	content := "apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\nresources:\n  - secret.yaml\n"
	require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(rendered, output)), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(rendered, output), []byte(content), 0o644))
	require.NoError(t, validateMaterializedSecretMembership(cfg, []clusterAppAction{{Output: output, Content: content}}, artifacts, rendered))
}

func TestRenderSingleServiceRejectsConflictingGroupedArtifact(t *testing.T) {
	cfg := newAutoTestConfig("grouped-secret-conflict")
	repo, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	cfg.OpenCenter.GitOps.Repository.LocalDir = repo
	cfg.OpenCenter.Services["kube-prometheus-stack"] = &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{Enabled: true, Namespace: "monitoring"}}
	cfg.Secrets.Grafana.AdminPassword = "grafana-password"
	cfg.Secrets.ServiceSecrets = map[string]any{"kube_prometheus_stack": map[string]any{"admin_password": "different"}}

	err = RenderSingleService(cfg, "kube-prometheus-stack", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "conflicting secret key")
}

func TestRenderSingleServiceIgnoresUnrelatedMaterializedArtifacts(t *testing.T) {
	cfg := newAutoTestConfig("single-service-materialized-artifacts")
	repo, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	cfg.OpenCenter.GitOps.Repository.LocalDir = repo
	cfg.OpenCenter.Services["custom-service"] = &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{Enabled: true, Namespace: "custom"}}
	cfg.OpenCenter.Services["other-service"] = &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{Enabled: true, Namespace: "other"}}
	cfg.Secrets.ServiceSecrets = map[string]any{
		"custom_service": map[string]any{"token": "custom"},
		"other_service":  map[string]any{"token": "other"},
	}

	artifacts, err := secretartifacts.Plan(&cfg)
	require.NoError(t, err)
	require.Len(t, artifacts, 2)
	materializeSecretArtifacts(t, cfg, artifacts)

	require.NoError(t, RenderSingleService(cfg, "custom-service", false))
}

func TestFilterSingleServiceArtifactsRetainsActionMatchedAggregate(t *testing.T) {
	cfg := newAutoTestConfig("single-service-aggregate-artifact")
	repo, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	cfg.OpenCenter.GitOps.Repository.LocalDir = repo
	artifact := secretartifacts.Artifact{
		TargetService: "aggregate-service",
		Path:          "services/aggregate-service/secret.yaml",
	}
	actions := []clusterAppAction{{Output: "services/aggregate-service/kustomization.yaml"}}
	materializeSecretArtifacts(t, cfg, []secretartifacts.Artifact{artifact})

	filtered := filterSingleServiceArtifacts("selected-service", actions, []secretartifacts.Artifact{artifact})
	require.Len(t, filtered, 1)

	rendered := filepath.Join(repo, "rendered")
	output := filepath.Join(rendered, "services", "aggregate-service", "kustomization.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(output), 0o755))
	require.NoError(t, os.WriteFile(output, []byte("apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\nresources: []\n"), 0o644))
	err = validateMaterializedSecretMembership(cfg, actions, filtered, rendered)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not include secret.yaml")
}

func kustomizationContent(actions []clusterAppAction) string {
	for _, action := range actions {
		if strings.HasSuffix(action.Output, "/kustomization.yaml") {
			return action.Content
		}
	}
	return ""
}
