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

package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/opencenter-cloud/opencenter-cli/internal/cluster"
	"github.com/opencenter-cloud/opencenter-cli/internal/config"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	"github.com/opencenter-cloud/opencenter-cli/internal/di"
	"github.com/opencenter-cloud/opencenter-cli/internal/logging"
	"github.com/opencenter-cloud/opencenter-cli/internal/secretartifacts"
	"github.com/spf13/cobra"
)

func newClusterDeployCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy [name]",
		Short: "Deploy a cluster from its openCenter configuration",
		Long: `Deploy a cluster from its openCenter configuration.

This command provisions infrastructure and deploys Kubernetes based on the
cluster configuration. The process is resumable; if a step fails, fix the issue
and re-run deploy to continue from the saved state.`,
		Args: cobra.MaximumNArgs(1),
		RunE: runClusterDeploy,
	}

	cmd.Flags().String("kubeconfig", "", "path to kubeconfig used by deploy actions (defaults to the cluster-owned kubeconfig path)")
	cmd.Flags().String("log", "", "log file path (defaults to <state_dir>/logs/bootstrap/<org>/<name>/bootstrap-YYYYMMDDTHHMMSSZ.log)")
	cmd.Flags().String("container-runtime", "", "container runtime for kind clusters (docker or podman)")
	cmd.Flags().Bool("restart", false, "rerun all deploy steps and ignore saved state")
	cmd.Flags().Bool("debug", false, "print deploy step debug details before each step runs")
	cmd.Flags().String("step", "", "run a single deploy step by ID")
	cmd.Flags().String("from-step", "", "restart deploy from the specified step ID")
	cmd.Flags().Bool("break-lock", false, "force removal of an existing operation lock before deploying")

	return cmd
}

func runClusterDeploy(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Apply log-level flag (already parsed by PersistentPreRunE, but ensure
	// it is honoured when the caller passes --log-level explicitly).
	if logLevel, _ := cmd.Flags().GetString("log-level"); logLevel != "" {
		_ = logging.SetLogLevel(logLevel)
	}

	// Resolve cluster name from args or active cluster
	name, err := resolveClusterNameForCommand(cmd, args, true)
	if err != nil {
		return err
	}

	// Reject planned providers that are not yet available
	cfg, err := loadCanonicalConfig(name)
	if err == nil {
		if err := checkProviderAvailability(cfg.OpenCenter.Infrastructure.Provider); err != nil {
			return err
		}
	}

	// Extract just the cluster name (without organization prefix) for path resolution
	actualClusterName := extractClusterName(name)
	organization := ""
	if err == nil {
		organization = cfg.OpenCenter.Meta.Organization
	}

	// Pre-check: ensure all secret manifests are encrypted before deploying.
	// This catches unencrypted secrets early, before any infrastructure changes.
	// DISABLED: key scanning hook temporarily disabled.
	// if err == nil {
	// 	gitDir := strings.TrimSpace(cfg.OpenCenter.GitOps.Repository.LocalDir)
	// 	if gitDir != "" {
	// 		if unencryptedErr := checkUnencryptedSecrets(gitDir); unencryptedErr != nil {
	// 			return unencryptedErr
	// 		}
	// 	}
	// }

	// Parse command-line options
	opts, err := parseBootstrapOptions(cmd, args, actualClusterName)
	if err != nil {
		return err
	}
	opts.Organization = organization

	if !opts.DryRun {
		// Acquire lock for deploy operation (with prompt if lock exists)
		lockResult, err := AcquireLockWithPrompt(ctx, cmd, name, "deploy", 1*time.Hour, map[string]string{
			"operation": "deploy",
			"command":   "cluster deploy",
		})
		if err != nil {
			return err
		}
		defer func() {
			if lockResult.Lock != nil {
				lockResult.LockManager.Release(lockResult.Lock)
			}
		}()
	}

	app, err := di.NewApp(config.ResolveClustersDir())
	if err != nil {
		return fmt.Errorf("initialize application graph: %w", err)
	}
	bootstrapService := app.BootstrapService
	bootstrapService.SetOutput(cmd.OutOrStdout())

	// Pre-check: ensure required secret manifests are synced and wired before
	// bootstrap, so secret-consuming services (e.g. Mimir) do not fail with
	// "secret ... not found" after a full deploy (OCTR-787).
	if !opts.DryRun && err == nil {
		if secretErr := checkSecretsSynced(&cfg, actualClusterName); secretErr != nil {
			return secretErr
		}
	}

	// Pre-check: ensure the GitOps working tree is clean before bootstrap.
	// A dirty tree causes git pull --rebase to fail during the gitea-rebase step.
	if !opts.DryRun {
		if gitDir := strings.TrimSpace(cfg.OpenCenter.GitOps.Repository.LocalDir); gitDir != "" {
			warnUncommittedChanges(ctx, cmd, gitDir)
			// Verify the local repo's origin remote points to git_url so the
			// gitea-rebase and gitops-push steps operate against the expected remote.
			if gitURL := strings.TrimSpace(cfg.OpenCenter.GitOps.Repository.URL); gitURL != "" {
				if err := verifyOriginMatchesGitURL(ctx, gitDir, gitURL); err != nil {
					return err
				}
			}
		}
	}

	if !opts.DryRun {
		if err := config.UpdateStatus(name, v2.StageBootstrap, v2.StatusRunning); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to update cluster status: %v\n", err)
		}
	}

	// Execute deploy
	result, err := bootstrapService.Bootstrap(ctx, opts)
	if err != nil {
		if !opts.DryRun {
			if statusErr := config.UpdateStatus(name, v2.StageBootstrap, v2.StatusFailed); statusErr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to update cluster status: %v\n", statusErr)
			}
		}
		if !opts.DryRun && result != nil {
			if strings.TrimSpace(result.LogPath) != "" {
				fmt.Fprintf(cmd.ErrOrStderr(), "Bootstrap log: %s\n", result.LogPath)
			}
			if strings.TrimSpace(result.ResumeStatePath) != "" {
				fmt.Fprintf(cmd.ErrOrStderr(), "Resume state: %s\n", result.ResumeStatePath)
			}
		}
		return err
	}

	if opts.DryRun {
		printClusterDeployPlan(cmd.OutOrStdout(), result.Plan)
		return nil
	}

	// Display results
	fmt.Fprintf(cmd.OutOrStdout(), "Deploy complete in %v\n", result.Duration.Round(time.Second))
	if result.Endpoint != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "Cluster endpoint: %s\n", result.Endpoint)
	}
	if strings.TrimSpace(result.LogPath) != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "Bootstrap log: %s\n", result.LogPath)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Steps completed: %d\n", len(result.StepsCompleted))
	if len(result.StepsFailed) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "Steps failed: %d\n", len(result.StepsFailed))
	}

	if !opts.DryRun {
		if err := config.UpdateStatus(name, v2.StageBootstrap, v2.StatusSuccess); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to update cluster status: %v\n", err)
		}
		printPostDeployNextSteps(cmd, name)
	}

	return nil
}

// setupBootstrapContainer initializes the DI container with all required services
func setupBootstrapContainer(container di.Container) error {
	pathResolver, err := di.ProvidePathResolver(config.ResolveClustersDir())
	if err != nil {
		return err
	}
	if err := container.Singleton("path-resolver", func() (*paths.PathResolver, error) {
		return pathResolver, nil
	}); err != nil {
		return err
	}
	if err := container.Singleton("validation-engine", di.ProvideValidationEngine); err != nil {
		return err
	}
	if err := container.Singleton("bootstrap-service", di.ProvideBootstrapService); err != nil {
		return err
	}
	return container.Initialize()
}

// parseBootstrapOptions parses command-line flags into BootstrapOptions
func parseBootstrapOptions(cmd *cobra.Command, args []string, clusterName string) (cluster.BootstrapOptions, error) {
	opts := cluster.BootstrapOptions{
		ClusterName: clusterName,
	}

	// Parse flags
	opts.DryRun = getGlobalOptions(cmd).DryRun
	opts.KubeconfigPath, _ = cmd.Flags().GetString("kubeconfig")
	opts.LogPath, _ = cmd.Flags().GetString("log")
	opts.ContainerRuntime, _ = cmd.Flags().GetString("container-runtime")
	opts.Restart, _ = cmd.Flags().GetBool("restart")
	opts.Debug, _ = cmd.Flags().GetBool("debug")
	opts.OnlyStep, _ = cmd.Flags().GetString("step")
	opts.FromStep, _ = cmd.Flags().GetString("from-step")

	// Validate mutually exclusive flags
	if strings.TrimSpace(opts.OnlyStep) != "" && strings.TrimSpace(opts.FromStep) != "" {
		return opts, fmt.Errorf("--step and --from-step cannot be used together")
	}

	return opts, nil
}

// printPostDeployNextSteps prints next steps after a successful deploy.
// Secrets are synced BEFORE deploy (enforced by checkSecretsSynced and the
// generate -> secrets sync -> deploy flow), so they are intentionally not
// listed here — only committing and pushing the generated tree remain.
func printPostDeployNextSteps(cmd *cobra.Command, name string) {
	fmt.Fprintln(cmd.OutOrStdout(), "\nNext steps:")
	fmt.Fprintf(cmd.OutOrStdout(), "  1. Commit changes:  git add -A && git commit -m \"deploy %s\"\n", name)
	fmt.Fprintln(cmd.OutOrStdout(), "  2. Push to remote:  git push")
}

// warnUncommittedChanges prints a warning if the GitOps directory has
// uncommitted changes. It does not commit or abort — the user is responsible
// for committing and pushing after deploy completes.
func warnUncommittedChanges(ctx context.Context, cmd *cobra.Command, gitDir string) {
	statusCmd := exec.CommandContext(ctx, "git", "-C", gitDir, "status", "--porcelain")
	output, err := statusCmd.Output()
	if err != nil {
		return
	}
	if len(strings.TrimSpace(string(output))) == 0 {
		return
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Warning: GitOps directory has uncommitted changes:\n%s\n", strings.TrimRight(string(output), "\n"))
}

// checkSecretsSynced verifies that every secret manifest the cluster
// configuration requires has been materialized by `opencenter secrets sync` AND
// wired into its service overlay kustomization's resources list. Deploying
// without this leaves services that consume the secret (e.g. Mimir's
// opencenter-mimir-secret) stuck in CreateContainerConfigError with
// "secret <name> not found" (OCTR-787). Failing fast here — before any
// infrastructure changes — gives the operator a clear, actionable next step
// instead of a cryptic pod-level failure after a full deploy.
//
// The set of required secrets is derived from secretartifacts.Plan(cfg), the
// same planner secrets sync uses, so this never hardcodes a service list.
func checkSecretsSynced(cfg *v2.Config, clusterName string) error {
	gitDir := strings.TrimSpace(cfg.OpenCenter.GitOps.Repository.LocalDir)
	if gitDir == "" {
		return nil
	}
	artifacts, err := secretartifacts.Plan(cfg)
	if err != nil {
		// Planning issues surface elsewhere; do not block deploy on them here.
		return nil
	}
	overlay := filepath.Join(gitDir, "applications", "overlays", clusterName)
	var missing []string
	for _, artifact := range artifacts {
		rel := filepath.FromSlash(artifact.Path) // services/<svc>/secret.yaml
		full := filepath.Join(overlay, rel)
		if _, statErr := os.Stat(full); statErr != nil {
			missing = append(missing, artifact.Path+" (not created)")
			continue
		}
		wired, wErr := secretWiredIntoKustomization(filepath.Dir(full))
		if wErr == nil && !wired {
			missing = append(missing, artifact.Path+" (created but not wired into kustomization)")
		}
	}
	if len(missing) == 0 {
		return nil
	}
	sort.Strings(missing)
	return fmt.Errorf(
		"required secret manifests are not ready for deploy:\n  %s\n\n"+
			"Run 'opencenter secrets sync %s' before deploying so these secrets are "+
			"created and applied by Flux; otherwise the consuming services fail with "+
			"\"secret ... not found\".",
		strings.Join(missing, "\n  "), clusterName)
}

// secretWiredIntoKustomization reports whether the kustomization.yaml in dir
// lists secret.yaml in its resources so Flux actually applies it.
func secretWiredIntoKustomization(dir string) (bool, error) {
	data, err := os.ReadFile(filepath.Join(dir, "kustomization.yaml"))
	if err != nil {
		return false, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		trimmed = strings.TrimPrefix(trimmed, "- ")
		trimmed = strings.Trim(trimmed, "\"'")
		trimmed = strings.TrimPrefix(trimmed, "./")
		if trimmed == "secret.yaml" {
			return true, nil
		}
	}
	return false, nil
}

// verifyOriginMatchesGitURL checks that the "origin" remote in gitDir points to
// the expected git_url from the cluster configuration. A mismatch means the
// rebase and push steps would operate against the wrong repository.
func verifyOriginMatchesGitURL(ctx context.Context, gitDir, expectedURL string) error {
	remoteCmd := exec.CommandContext(ctx, "git", "-C", gitDir, "remote", "get-url", "origin")
	output, err := remoteCmd.Output()
	if err != nil {
		// No origin remote — the bootstrap steps will add it, so skip the check.
		return nil
	}
	actual := strings.TrimSpace(string(output))
	if actual != expectedURL {
		return fmt.Errorf("git remote origin in %s points to %q, but git_url is %q\nUpdate the remote with: git -C %s remote set-url origin %s",
			gitDir, actual, expectedURL, gitDir, expectedURL)
	}
	return nil
}
