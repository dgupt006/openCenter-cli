package cluster

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

const openStackFluxBootstrapStepID = "openstack-flux-bootstrap"
const gitOpsPublishStepID = "gitops-publish"

const openStackCloudProviderToleration = "node.cloudprovider.kubernetes.io/uninitialized"

// buildFluxBootstrapStep returns a bootstrap step that runs `flux bootstrap`
// against the newly provisioned OpenStack cluster. The provider-specific
// command (github, gitea, or gitlab) is chosen based on
// opencenter.gitops.auth.token.provider.
func (p *openstackBootstrapProvider) buildFluxBootstrapStep(
	cfg *v2.Config,
	clusterDir string,
	planEnv []BootstrapPlanEnv,
	opts *BootstrapOptions,
) (bootstrapStep, error) {
	params, err := resolveFluxBootstrapParams(cfg)
	if err != nil {
		return bootstrapStep{}, fmt.Errorf("resolving flux bootstrap parameters: %w", err)
	}

	planCommands := fluxBootstrapPlanCommands(params)

	return bootstrapStep{
		ID:          openStackFluxBootstrapStepID,
		Description: fmt.Sprintf("Bootstrap FluxCD using %s provider", params.Provider),
		Plan: BootstrapPlanStep{
			ID:          openStackFluxBootstrapStepID,
			Action:      fmt.Sprintf("Bootstrap FluxCD using %s provider", params.Provider),
			WorkingDir:  clusterDir,
			Commands:    planCommands,
			Environment: planEnv,
			Reads:       []string{opts.KubeconfigPath},
			Writes:      []string{"FluxCD resources in flux-system namespace"},
			Notes:       []string{"Plan only; flux CLI availability, token validity, and Kubernetes API access were not checked."},
		},
		Run: func(ctx context.Context) error {
			return p.runFluxBootstrap(ctx, cfg, opts.KubeconfigPath)
		},
	}, nil
}

// buildGitOpsPublishStep publishes the complete generated cluster tree before
// Flux is bootstrapped. Flux only commits its own bootstrap objects; it does
// not publish the manifests rendered by opencenter. Leaving that gap to a
// human caused clusters where Flux was healthy but no applications existed.
func (p *openstackBootstrapProvider) buildGitOpsPublishStep(cfg *v2.Config, clusterDir string, planEnv []BootstrapPlanEnv) (bootstrapStep, error) {
	if strings.TrimSpace(cfg.GitDir()) == "" {
		return bootstrapStep{}, fmt.Errorf("opencenter.gitops.repository.local_dir must be configured before GitOps publish")
	}
	if cfg.ConfiguredGitURL() == "" {
		return bootstrapStep{}, fmt.Errorf("opencenter.gitops.repository.url must be configured before GitOps publish")
	}
	return bootstrapStep{
		ID:          gitOpsPublishStepID,
		Description: "Publish generated GitOps cluster tree",
		Plan: BootstrapPlanStep{
			ID:         gitOpsPublishStepID,
			Action:     "Publish generated GitOps cluster tree",
			WorkingDir: cfg.GitDir(),
			Commands: []BootstrapPlanCommand{
				commandPlan("git", "add", "applications/overlays/"+cfg.ClusterName(), "clusters/"+cfg.ClusterName(), "infrastructure/clusters/"+cfg.ClusterName(), ".opencenter/ownership/clusters/"+cfg.ClusterName()),
				commandPlan("git", "commit", "-m", "publish generated cluster tree"),
				commandPlan("git", "push", "origin", "HEAD:"+cfg.GitBranchOrDefault()),
			},
			Environment: planEnv,
			Reads:       []string{cfg.GitDir()},
			Writes:      []string{"configured GitOps remote branch"},
			Notes:       []string{"The generated cluster tree is committed and pushed before Flux bootstrap; no manual GitOps publication is required."},
		},
		Run: func(ctx context.Context) error {
			return p.publishGitOpsTree(ctx, cfg)
		},
	}, nil
}

func (p *openstackBootstrapProvider) publishGitOpsTree(ctx context.Context, cfg *v2.Config) error {
	gitDir := filepath.Clean(cfg.GitDir())
	if _, err := os.Stat(filepath.Join(gitDir, ".git")); err != nil {
		return fmt.Errorf("GitOps local_dir %q is not a Git checkout: %w", gitDir, err)
	}
	token, err := resolveFluxToken(cfg)
	if err != nil {
		return fmt.Errorf("resolve GitOps push credentials: %w", err)
	}
	askPass, err := os.CreateTemp("", "opencenter-git-askpass-*")
	if err != nil {
		return fmt.Errorf("create temporary Git credential helper: %w", err)
	}
	askPath := askPass.Name()
	defer os.Remove(askPath)
	if err := askPass.Chmod(0o700); err != nil {
		askPass.Close()
		return fmt.Errorf("protect temporary Git credential helper: %w", err)
	}
	if _, err := askPass.WriteString("#!/bin/sh\ncase \"$1\" in *Username*) printf '%s\\n' x-access-token ;; *) printf '%s\\n' \"$GIT_PASSWORD\" ;; esac\n"); err != nil {
		askPass.Close()
		return fmt.Errorf("write temporary Git credential helper: %w", err)
	}
	if err := askPass.Close(); err != nil {
		return fmt.Errorf("close temporary Git credential helper: %w", err)
	}
	env := map[string]string{
		"GIT_ASKPASS":         askPath,
		"GIT_PASSWORD":        token,
		"GIT_TERMINAL_PROMPT": "0",
		"GIT_AUTHOR_NAME":     "openCenter",
		"GIT_AUTHOR_EMAIL":    "opencenter@localhost",
		"GIT_COMMITTER_NAME":  "openCenter",
		"GIT_COMMITTER_EMAIL": "opencenter@localhost",
	}
	cluster := cfg.ClusterName()
	paths := []string{"applications/overlays/" + cluster, "clusters/" + cluster, "infrastructure/clusters/" + cluster, ".opencenter/ownership/clusters/" + cluster}
	status, statusErr := p.runner.Run(ctx, gitDir, env, "git", append([]string{"status", "--porcelain", "--untracked-files=all", "--"}, paths...)...)
	if statusErr != nil {
		return fmt.Errorf("inspect generated GitOps tree: %w", statusErr)
	}
	if strings.TrimSpace(string(status)) != "" {
		if _, err := p.runner.Run(ctx, gitDir, env, "git", append([]string{"add", "--"}, paths...)...); err != nil {
			return fmt.Errorf("stage generated GitOps tree: %w", err)
		}
		if _, err := p.runner.Run(ctx, gitDir, env, "git", "commit", "-m", "publish generated cluster tree"); err != nil {
			return fmt.Errorf("commit generated GitOps tree: %w", err)
		}
	}
	branch := cfg.GitBranchOrDefault()
	if _, err := p.runner.Run(ctx, gitDir, env, "git", "push", "origin", "HEAD:"+branch); err != nil {
		return fmt.Errorf("push generated GitOps tree to %s: %w", branch, err)
	}
	return nil
}

// fluxBootstrapParams holds the resolved parameters for a flux bootstrap command.
type fluxBootstrapParams struct {
	// Provider is the git provider: github, gitea, or gitlab.
	Provider string

	// Owner is the repository owner (organization or username).
	Owner string

	// Repository is the repository name (without owner prefix).
	Repository string

	// Branch is the target branch (defaults to main).
	Branch string

	// Path is the cluster path within the repository.
	Path string

	// TokenFile is the path to the file containing the access token.
	TokenFile string

	// Personal indicates that a GitHub owner is a user account rather than an
	// organization. It is ignored for non-GitHub providers.
	Personal bool
}

// resolveFluxBootstrapParams extracts and validates all parameters needed for
// flux bootstrap from the cluster configuration.
func resolveFluxBootstrapParams(cfg *v2.Config) (*fluxBootstrapParams, error) {
	if cfg.OpenCenter.GitOps.Auth.Token == nil {
		return nil, fmt.Errorf("gitops.auth.token must be configured for flux bootstrap; set opencenter.gitops.auth.token.provider to github, gitea, or gitlab")
	}

	provider := strings.ToLower(strings.TrimSpace(cfg.OpenCenter.GitOps.Auth.Token.Provider))
	if provider == "" {
		return nil, fmt.Errorf("gitops.auth.token.provider is required; set it to github, gitea, or gitlab")
	}

	switch provider {
	case "github", "gitea", "gitlab":
		// valid
	default:
		return nil, fmt.Errorf("unsupported gitops.auth.token.provider %q; supported values: github, gitea, gitlab", provider)
	}

	repoURL := cfg.ConfiguredGitURL()
	if repoURL == "" {
		return nil, fmt.Errorf("gitops.repository.url must be configured for flux bootstrap")
	}

	owner, repo, err := parseFluxGitURL(repoURL)
	if err != nil {
		return nil, fmt.Errorf("parsing repository URL %q: %w", repoURL, err)
	}

	// Owner from token config takes precedence over URL-derived owner.
	if configOwner := strings.TrimSpace(cfg.OpenCenter.GitOps.Auth.Token.Owner); configOwner != "" {
		owner = configOwner
	}
	if owner == "" {
		return nil, fmt.Errorf("repository owner could not be determined from URL %q; set opencenter.gitops.auth.token.owner explicitly", repoURL)
	}

	branch := cfg.GitBranchOrDefault()

	clusterName := cfg.ClusterName()
	if clusterName == "" {
		return nil, fmt.Errorf("cluster name must be set for flux bootstrap path")
	}
	bootstrapPath := "clusters/" + clusterName

	// Allow the user to override the path via gitops.repository.path.
	if configPath := strings.TrimSpace(cfg.OpenCenter.GitOps.Repository.Path); configPath != "" {
		bootstrapPath = configPath
	}

	tokenFile := strings.TrimSpace(cfg.OpenCenter.GitOps.Auth.Token.TokenFile)
	if tokenFile == "" {
		// Token may be inline; we'll resolve it at runtime.
		if strings.TrimSpace(cfg.OpenCenter.GitOps.Auth.Token.Token) == "" {
			return nil, fmt.Errorf("gitops.auth.token.token_file or gitops.auth.token.token must be configured for flux bootstrap")
		}
	}

	return &fluxBootstrapParams{
		Provider:   provider,
		Owner:      owner,
		Repository: repo,
		Branch:     branch,
		Path:       bootstrapPath,
		TokenFile:  tokenFile,
		Personal:   cfg.OpenCenter.GitOps.Auth.Token.Personal,
	}, nil
}

// runFluxBootstrap executes the flux bootstrap command for the configured
// git provider.
func (p *openstackBootstrapProvider) runFluxBootstrap(ctx context.Context, cfg *v2.Config, kubeconfigPath string) error {
	params, err := resolveFluxBootstrapParams(cfg)
	if err != nil {
		return err
	}

	token, err := resolveFluxToken(cfg)
	if err != nil {
		return err
	}

	env := buildBootstrapEnvironment(kubeconfigPath)

	var fluxArgs []string

	switch params.Provider {
	case "github":
		fluxArgs = []string{
			"bootstrap", "github",
			"--token-auth",
			"--toleration-keys=" + openStackCloudProviderToleration,
			"--owner=" + params.Owner,
			"--repository=" + params.Repository,
			"--branch=" + params.Branch,
			"--path=" + params.Path,
		}
		if params.Personal {
			fluxArgs = append(fluxArgs, "--personal")
		}
		env["GITHUB_TOKEN"] = token

	case "gitea":
		fluxArgs = []string{
			"bootstrap", "gitea",
			"--token-auth",
			"--toleration-keys=" + openStackCloudProviderToleration,
			"--owner=" + params.Owner,
			"--repository=" + params.Repository,
			"--branch=" + params.Branch,
			"--path=" + params.Path,
		}
		env["GITEA_TOKEN"] = token

	case "gitlab":
		fluxArgs = []string{
			"bootstrap", "gitlab",
			"--token-auth",
			"--toleration-keys=" + openStackCloudProviderToleration,
			"--owner=" + params.Owner,
			"--repository=" + params.Repository,
			"--branch=" + params.Branch,
			"--path=" + params.Path,
		}
		env["GITLAB_TOKEN"] = token

	default:
		return fmt.Errorf("unsupported git provider: %q", params.Provider)
	}

	gitDir := cfg.GitDir()
	if gitDir == "" {
		gitDir = "."
	}

	_, runErr := p.runner.Run(ctx, gitDir, env, "flux", fluxArgs...)
	if runErr != nil {
		return fmt.Errorf("flux bootstrap %s: %w", params.Provider, runErr)
	}

	return nil
}

// resolveFluxToken reads the access token from the configured token file or
// returns the inline token value.
func resolveFluxToken(cfg *v2.Config) (string, error) {
	if cfg.OpenCenter.GitOps.Auth.Token == nil {
		return "", fmt.Errorf("gitops.auth.token is not configured")
	}

	tokenFile := strings.TrimSpace(cfg.OpenCenter.GitOps.Auth.Token.TokenFile)
	if tokenFile != "" {
		data, err := os.ReadFile(tokenFile)
		if err != nil {
			return "", fmt.Errorf("reading token file %s: %w", tokenFile, err)
		}
		token := strings.TrimSpace(string(data))
		if token == "" {
			return "", fmt.Errorf("token file %s is empty", tokenFile)
		}
		return token, nil
	}

	inlineToken := strings.TrimSpace(cfg.OpenCenter.GitOps.Auth.Token.Token)
	if inlineToken != "" {
		if inlineToken == "CHANGEME" {
			return "", fmt.Errorf("gitops.auth.token.token is still set to the placeholder value; replace it with a real access token")
		}
		return inlineToken, nil
	}

	return "", fmt.Errorf("no token available: set gitops.auth.token.token_file or gitops.auth.token.token")
}

// parseFluxGitURL extracts owner and repository from various Git URL formats.
// Supports SSH (git@host:owner/repo.git), HTTPS, and ssh:// formats.
func parseFluxGitURL(repoURL string) (owner, repo string, err error) {
	repoURL = strings.TrimSpace(repoURL)
	if repoURL == "" {
		return "", "", fmt.Errorf("empty repository URL")
	}

	// Handle SSH format: git@host:owner/repo.git
	if strings.HasPrefix(repoURL, "git@") {
		parts := strings.SplitN(repoURL, ":", 2)
		if len(parts) != 2 {
			return "", "", fmt.Errorf("invalid SSH URL format: %s", repoURL)
		}
		pathPart := strings.TrimSuffix(parts[1], ".git")
		return splitFluxOwnerRepo(pathPart)
	}

	// Handle URL formats: https://... or ssh://...
	parsed, err := url.Parse(repoURL)
	if err != nil {
		return "", "", fmt.Errorf("parsing URL %s: %w", repoURL, err)
	}

	urlPath := strings.TrimPrefix(parsed.Path, "/")
	urlPath = strings.TrimSuffix(urlPath, ".git")

	return splitFluxOwnerRepo(urlPath)
}

// splitFluxOwnerRepo splits "owner/repo" or "owner/group/repo" into owner and
// repo. For nested groups (GitLab), the full path before the last segment is
// the owner.
func splitFluxOwnerRepo(pathPart string) (owner, repo string, err error) {
	pathPart = strings.Trim(pathPart, "/")
	if pathPart == "" {
		return "", "", fmt.Errorf("empty path in URL")
	}

	parts := strings.Split(pathPart, "/")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("URL path %q does not contain owner/repo", pathPart)
	}

	repo = parts[len(parts)-1]
	owner = path.Join(parts[:len(parts)-1]...)

	return owner, repo, nil
}

// fluxBootstrapPlanCommands returns the planned commands for the dry-run output.
func fluxBootstrapPlanCommands(params *fluxBootstrapParams) []BootstrapPlanCommand {
	switch params.Provider {
	case "github":
		args := []string{
			"bootstrap", "github",
			"--token-auth",
			"--toleration-keys=" + openStackCloudProviderToleration,
			"--owner=" + params.Owner,
			"--repository=" + params.Repository,
			"--branch=" + params.Branch,
			"--path=" + params.Path,
		}
		if params.Personal {
			args = append(args, "--personal")
		}
		return []BootstrapPlanCommand{commandPlan("flux", args...)}
	case "gitea":
		return []BootstrapPlanCommand{commandPlan("flux",
			"bootstrap", "gitea",
			"--token-auth",
			"--toleration-keys="+openStackCloudProviderToleration,
			"--owner="+params.Owner,
			"--repository="+params.Repository,
			"--branch="+params.Branch,
			"--path="+params.Path,
		)}
	case "gitlab":
		return []BootstrapPlanCommand{commandPlan("flux",
			"bootstrap", "gitlab",
			"--token-auth",
			"--toleration-keys="+openStackCloudProviderToleration,
			"--owner="+params.Owner,
			"--repository="+params.Repository,
			"--branch="+params.Branch,
			"--path="+params.Path,
		)}
	default:
		return nil
	}
}
