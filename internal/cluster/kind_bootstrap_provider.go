// Copyright 2025 Victor Palma <victor.palma@rackspace.com>
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cluster

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	kindprovider "github.com/opencenter-cloud/opencenter-cli/internal/cloud/kind"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	"github.com/opencenter-cloud/opencenter-cli/internal/localdev"
	"github.com/opencenter-cloud/opencenter-cli/internal/localdev/flux"
	"github.com/opencenter-cloud/opencenter-cli/internal/localdev/gitea"
	"github.com/opencenter-cloud/opencenter-cli/internal/localdev/gitops"
)

type kindBootstrapProvider struct {
	runner lifecycleCommandRunner
}

func newKindBootstrapProvider(runner lifecycleCommandRunner) lifecycleBootstrapProvider {
	return &kindBootstrapProvider{runner: runner}
}

// BuildSteps returns the ordered bootstrap steps for a Kind cluster.
//
// The sequence mirrors the production OpenStack workflow: provision
// infrastructure, attach the Git remote, push the GitOps tree, and
// bootstrap FluxCD — all within a single `opencenter cluster deploy`
// invocation.
func (p *kindBootstrapProvider) BuildSteps(cfg *v2.Config, clusterPaths *paths.ClusterPaths, opts *BootstrapOptions) ([]bootstrapStep, error) {
	kindProvider := kindprovider.NewProvider()
	runtime := kindprovider.ResolveRuntime(opts.ContainerRuntime)
	env := kindprovider.BuildEnvironment(runtime)
	kindConfigPath := filepath.Join(clusterPaths.ClusterDir, "kind-config.yaml")
	kindClusterName := resolveKindClusterName(cfg)

	executor := localdev.NewExecutor()
	// stateDir is empty so the localdev layout resolves to <OPENCENTER_CONFIG_DIR>/local.
	stateDir := ""

	clusterIdentifier := cfg.ClusterName()
	if org := strings.TrimSpace(cfg.Organization()); org != "" {
		clusterIdentifier = org + "/" + clusterIdentifier
	}
	gitDir, gitDirErr := resolveGitDir(cfg, clusterPaths)

	return []bootstrapStep{
		{
			ID:          "kind-create",
			Description: "Create Kind cluster",
			Plan: BootstrapPlanStep{
				ID:         "kind-create",
				Action:     "Create Kind cluster",
				WorkingDir: clusterPaths.ClusterDir,
				Commands: []BootstrapPlanCommand{
					commandPlan("kind", "get", "clusters"),
					commandPlan("kind", "create", "cluster", "--name", kindClusterName, "--config", kindConfigPath),
				},
				Reads:       []string{kindConfigPath},
				Writes:      []string{fmt.Sprintf("local Kind cluster %q", kindClusterName)},
				Environment: kindPlanEnv(env),
				Notes:       []string{"Plan only; kind availability and config file existence were not checked."},
			},
			Run: func(ctx context.Context) error {
				if _, err := os.Stat(kindConfigPath); err != nil {
					if os.IsNotExist(err) {
						return fmt.Errorf("kind config not found at %s; run 'opencenter cluster generate %s' first", kindConfigPath, cfg.ClusterName())
					}
					return fmt.Errorf("stat kind config: %w", err)
				}
				return kindProvider.CreateCluster(ctx, kindClusterName, kindConfigPath, env)
			},
		},
		{
			ID:          "kind-export-kubeconfig",
			Description: "Export Kind kubeconfig",
			Plan: BootstrapPlanStep{
				ID:         "kind-export-kubeconfig",
				Action:     "Export Kind kubeconfig",
				WorkingDir: clusterPaths.ClusterDir,
				Commands: []BootstrapPlanCommand{
					commandPlan("kind", "export", "kubeconfig", "--name", kindClusterName, "--kubeconfig", opts.KubeconfigPath),
				},
				Writes:      []string{filepath.Dir(opts.KubeconfigPath), opts.KubeconfigPath},
				Environment: kindPlanEnv(env),
				Notes:       []string{"Plan only; kubeconfig directory creation and kind availability were not checked."},
			},
			Run: func(ctx context.Context) error {
				return kindProvider.ExportKubeconfig(ctx, kindClusterName, opts.KubeconfigPath, env)
			},
		},
		{
			ID:          "gitea-attach-kind",
			Description: "Attach local Gitea to the Kind network",
			Plan: BootstrapPlanStep{
				ID:          "gitea-attach-kind",
				Action:      "Attach local Gitea to the Kind network",
				WorkingDir:  clusterPaths.ClusterDir,
				Environment: kindPlanEnv(env),
				Writes:      []string{"local Gitea container network attachments"},
				Notes:       []string{"Plan only; local Gitea status and container runtime availability were not checked."},
			},
			Run: func(ctx context.Context) error {
				if os.Getenv("OPENCENTER_TEST_MODE") != "" {
					return nil
				}
				giteaService, err := gitea.NewService(executor, stateDir, gitea.DefaultSettings(runtime))
				if err != nil {
					return fmt.Errorf("create gitea service: %w", err)
				}
				status, err := giteaService.Status(ctx)
				if err != nil {
					return fmt.Errorf("check gitea status: %w", err)
				}
				if !status.Running {
					return fmt.Errorf("local gitea is not running; run 'opencenter local gitea up' first")
				}
				if _, err := giteaService.AttachKind(ctx); err != nil {
					return fmt.Errorf("attach gitea to kind network: %w", err)
				}
				return nil
			},
		},
		{
			ID:          "flux-bootstrap",
			Description: "Bootstrap FluxCD from local Gitea",
			Plan: BootstrapPlanStep{
				ID:         "flux-bootstrap",
				Action:     "Bootstrap FluxCD from local Gitea",
				WorkingDir: gitDir,
				Commands: []BootstrapPlanCommand{
					commandPlan("flux", "bootstrap", "<provider-specific>", "--path=applications/overlays/"+cfg.ClusterName()),
				},
				Environment: []BootstrapPlanEnv{{Name: "KUBECONFIG", Value: opts.KubeconfigPath}},
				Writes:      []string{"Flux bootstrap manifests and commits in the GitOps repository", "Flux resources in the Kind cluster"},
				Notes:       appendPlanNotes([]string{"Plan only; local Gitea status, token files, current branch, and kubeconfig were not checked."}, gitDirErr),
			},
			Run: func(ctx context.Context) error {
				if os.Getenv("OPENCENTER_TEST_MODE") != "" {
					return nil
				}
				fluxService, err := flux.NewService(executor, stateDir)
				if err != nil {
					return fmt.Errorf("create flux service: %w", err)
				}
				if _, err := fluxService.Bootstrap(ctx, clusterIdentifier); err != nil {
					return fmt.Errorf("flux bootstrap: %w", err)
				}
				return nil
			},
		},
		{
			ID:          baseRepoSecretStepID,
			Description: "Reconcile base Git repository credentials for Flux",
			Plan:        newBaseRepoSecretStep(cfg, opts.KubeconfigPath, p.runner).Plan,
			Run: func(ctx context.Context) error {
				if os.Getenv("OPENCENTER_TEST_MODE") != "" {
					return nil
				}
				return reconcileBaseRepoSecret(ctx, cfg, opts.KubeconfigPath, p.runner)
			},
		},
		{
			ID:          sopsAgeSecretStepID,
			Description: "Reconcile SOPS Age decryption key for Flux",
			Plan:        newSopsAgeSecretStep(clusterPaths.SOPSKeyPath, opts.KubeconfigPath, p.runner).Plan,
			Run: func(ctx context.Context) error {
				if os.Getenv("OPENCENTER_TEST_MODE") != "" {
					return nil
				}
				return reconcileSopsAgeSecret(ctx, clusterPaths.SOPSKeyPath, opts.KubeconfigPath, p.runner)
			},
		},
		{
			ID:          "gitea-rebase",
			Description: "Rebase local checkout with Flux bootstrap commits from Gitea",
			Plan: BootstrapPlanStep{
				ID:         "gitea-rebase",
				Action:     "Rebase local checkout with Flux bootstrap commits from Gitea",
				WorkingDir: gitDir,
				Commands: []BootstrapPlanCommand{
					commandPlan("git", "status", "--porcelain"),
					commandPlan("git", "add", "-A"),
					commandPlan("git", "commit", "-m", "stage local changes before rebase"),
					commandPlan("git", "pull", "--rebase", "origin", "<current-branch>"),
				},
				Reads:  []string{gitDir},
				Writes: []string{"local GitOps checkout history and working tree"},
				Notes:  appendPlanNotes([]string{"Plan only; Git remotes, token files, branch, and local working tree state were not checked."}, gitDirErr),
			},
			Run: func(ctx context.Context) error {
				if os.Getenv("OPENCENTER_TEST_MODE") != "" {
					return nil
				}
				gitopsService, err := gitops.NewService(executor, stateDir)
				if err != nil {
					return fmt.Errorf("create gitops service: %w", err)
				}
				gitDir, err := resolveGitDir(cfg, clusterPaths)
				if err != nil {
					return fmt.Errorf("resolve git dir: %w", err)
				}
				if _, err := gitopsService.PullRebase(ctx, clusterIdentifier, gitDir); err != nil {
					return fmt.Errorf("rebase from gitea: %w", err)
				}
				return nil
			},
		},
		{
			ID:          "gitops-push",
			Description: "Push GitOps repository to local Gitea",
			Plan: BootstrapPlanStep{
				ID:         "gitops-push",
				Action:     "Push GitOps repository to local Gitea",
				WorkingDir: gitDir,
				Commands: []BootstrapPlanCommand{
					commandPlan("git", "push", "-u", "origin", "<current-branch>"),
				},
				Reads:  []string{gitDir},
				Writes: []string{"local Gitea Git repository"},
				Notes:  appendPlanNotes([]string{"Plan only; local Gitea status, token files, Git remote, and current branch were not checked."}, gitDirErr),
			},
			Run: func(ctx context.Context) error {
				if os.Getenv("OPENCENTER_TEST_MODE") != "" {
					return nil
				}
				gitopsService, err := gitops.NewService(executor, stateDir)
				if err != nil {
					return fmt.Errorf("create gitops service: %w", err)
				}
				if _, err := gitopsService.Push(ctx, clusterIdentifier); err != nil {
					return fmt.Errorf("push gitops repo: %w", err)
				}
				return nil
			},
		},
		{
			ID:          "flux-verify",
			Description: "Verify Flux installation and source reconciliation",
			Plan: BootstrapPlanStep{
				ID:         "flux-verify",
				Action:     "Verify Flux installation and source reconciliation",
				WorkingDir: clusterPaths.ClusterDir,
				Commands: []BootstrapPlanCommand{
					commandPlan("flux", "check"),
					commandPlan("flux", "get", "sources", "git", "-n", "flux-system"),
					commandPlan("flux", "get", "kustomizations", "-n", "flux-system"),
				},
				Environment: []BootstrapPlanEnv{{Name: "KUBECONFIG", Value: opts.KubeconfigPath}},
				Reads:       []string{opts.KubeconfigPath},
				Notes:       []string{"Plan only; Flux installation and source reconciliation were not checked."},
			},
			Run: func(ctx context.Context) error {
				if os.Getenv("OPENCENTER_TEST_MODE") != "" {
					return nil
				}
				kubeconfigEnv := map[string]string{"KUBECONFIG": opts.KubeconfigPath}

				// flux check — verifies Flux components are installed and healthy.
				if _, err := executor.Run(ctx, localdev.RunOptions{
					Name: "flux",
					Env:  kubeconfigEnv,
					Args: []string{"check"},
				}); err != nil {
					return fmt.Errorf("flux check: %w", err)
				}

				// flux get sources git — verifies GitRepository sources are reconciled.
				if _, err := executor.Run(ctx, localdev.RunOptions{
					Name: "flux",
					Env:  kubeconfigEnv,
					Args: []string{"get", "sources", "git", "-n", "flux-system"},
				}); err != nil {
					return fmt.Errorf("flux get sources git: %w", err)
				}

				// flux get kustomizations — verifies Kustomization objects are reconciled.
				if _, err := executor.Run(ctx, localdev.RunOptions{
					Name: "flux",
					Env:  kubeconfigEnv,
					Args: []string{"get", "kustomizations", "-n", "flux-system"},
				}); err != nil {
					return fmt.Errorf("flux get kustomizations: %w", err)
				}

				return nil
			},
		},
	}, nil
}

// resolveGitDir returns the GitOps directory for the cluster, falling back to
// the cluster paths when the config does not specify one.
func resolveGitDir(cfg *v2.Config, clusterPaths *paths.ClusterPaths) (string, error) {
	gitDir := strings.TrimSpace(cfg.GitDir())
	if gitDir == "" {
		gitDir = clusterPaths.GitOpsDir
	}
	if gitDir == "" {
		return "", fmt.Errorf("cluster %q does not define a git_dir", cfg.ClusterName())
	}
	return gitDir, nil
}

// resolveKindClusterName returns the Kind cluster name, preferring an explicit
// override from the config when set.
func resolveKindClusterName(cfg *v2.Config) string {
	return cfg.ClusterName()
}
