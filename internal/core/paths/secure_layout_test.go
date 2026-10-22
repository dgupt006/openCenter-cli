package paths

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPathResolverSecureLayoutCreatesScopedZones(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	clustersRoot := filepath.Join(root, "clusters")
	gitopsRoot := filepath.Join(root, "gitops-root")
	stateRoot := filepath.Join(root, "state-root")
	secretsRoot := filepath.Join(root, "secrets-root")

	resolver := NewPathResolverWithRoots(clustersRoot, filepath.Join(clustersRoot, "blueprints"), gitopsRoot, stateRoot, secretsRoot, DefaultResolutionOptions())
	if err := resolver.CreateClusterDirectories(context.Background(), "demo-cluster", "acme"); err != nil {
		t.Fatalf("CreateClusterDirectories() error = %v", err)
	}

	clusterPaths, err := resolver.Resolve(context.Background(), "demo-cluster", "acme")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	assertPath := func(label, got, want string) {
		t.Helper()
		if got != want {
			t.Fatalf("%s = %q, want %q", label, got, want)
		}
	}

	assertPath("OrganizationDir", clusterPaths.OrganizationDir, filepath.Join(gitopsRoot, "acme"))
	assertPath("GitOpsDir", clusterPaths.GitOpsDir, filepath.Join(gitopsRoot, "acme"))
	assertPath("ClusterDir", clusterPaths.ClusterDir, filepath.Join(gitopsRoot, "acme", "infrastructure", "clusters", "demo-cluster"))
	assertPath("ClusterStateDir", clusterPaths.ClusterStateDir, clusterPaths.ClusterDir)
	assertPath("SecretsDir", clusterPaths.SecretsDir, filepath.Join(secretsRoot, "acme", "demo-cluster"))
	assertPath("ConfigPath", clusterPaths.ConfigPath, filepath.Join(clustersRoot, "blueprints", "acme", "demo-cluster", "demo-cluster-config.yaml"))
	assertPath("KubeconfigPath", clusterPaths.KubeconfigPath, filepath.Join(secretsRoot, "acme", "demo-cluster", "kubernetes", "kubeconfig.yaml"))
	assertPath("InventoryPath", clusterPaths.InventoryPath, filepath.Join(clusterPaths.ClusterDir, "inventory"))
	assertPath("VenvPath", clusterPaths.VenvPath, filepath.Join(clusterPaths.ClusterDir, "venv"))
	assertPath("BinPath", clusterPaths.BinPath, filepath.Join(clusterPaths.ClusterDir, ".bin"))
	assertPath("ApplicationsDir", clusterPaths.ApplicationsDir, filepath.Join(gitopsRoot, "acme", "applications", "overlays", "demo-cluster"))
	assertPath("SOPSConfigPath", clusterPaths.SOPSConfigPath, filepath.Join(gitopsRoot, "acme", ".sops.yaml"))
	assertPath("SOPSKeyPath", clusterPaths.SOPSKeyPath, filepath.Join(secretsRoot, "acme", "demo-cluster", "age", "keys", "demo-cluster-key.txt"))
	assertPath("SSHKeyPath", clusterPaths.SSHKeyPath, filepath.Join(secretsRoot, "acme", "demo-cluster", "ssh", "demo-cluster"))

	if err := clusterPaths.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestClusterPathsValidateRejectsGitOpsContainment(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	gitopsDir := filepath.Join(root, "gitops", "acme")
	if err := os.MkdirAll(gitopsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	valid := ClusterPaths{
		GitOpsDir:       gitopsDir,
		ClusterDir:      filepath.Join(gitopsDir, "infrastructure", "clusters", "demo"),
		ClusterStateDir: filepath.Join(gitopsDir, "infrastructure", "clusters", "demo"),
		SecretsDir:      filepath.Join(root, "secrets", "acme", "demo"),
		ConfigPath:      filepath.Join(root, "blueprints", "acme", "demo", "demo-config.yaml"),
		KubeconfigPath:  filepath.Join(root, "secrets", "acme", "demo", "kubernetes", "kubeconfig.yaml"),
		SOPSKeyPath:     filepath.Join(root, "secrets", "acme", "demo", "age", "keys", "demo-key.txt"),
		SSHKeyPath:      filepath.Join(root, "secrets", "acme", "demo", "ssh", "demo"),
	}

	tests := []struct {
		name   string
		mutate func(*ClusterPaths)
	}{
		{
			name: "kubeconfig under gitops dir",
			mutate: func(p *ClusterPaths) {
				p.KubeconfigPath = filepath.Join(gitopsDir, "kubeconfig.yaml")
			},
		},
		{
			name: "config path under gitops dir",
			mutate: func(p *ClusterPaths) {
				p.ConfigPath = filepath.Join(gitopsDir, "demo-config.yaml")
			},
		},
		{
			name: "secrets dir under gitops dir",
			mutate: func(p *ClusterPaths) {
				p.SecretsDir = filepath.Join(gitopsDir, "secrets")
			},
		},
		{
			name: "sops key under gitops dir",
			mutate: func(p *ClusterPaths) {
				p.SOPSKeyPath = filepath.Join(gitopsDir, "keys.txt")
			},
		},
		{
			name: "ssh key under gitops dir",
			mutate: func(p *ClusterPaths) {
				p.SSHKeyPath = filepath.Join(gitopsDir, "id_ed25519")
			},
		},
		{
			name: "common prefix sibling stays valid",
			mutate: func(p *ClusterPaths) {
				p.SecretsDir = gitopsDir + "-secrets"
				p.ConfigPath = filepath.Join(gitopsDir+"-blueprints", "demo-config.yaml")
				p.KubeconfigPath = filepath.Join(gitopsDir+"-secrets", "kubeconfig.yaml")
				p.SOPSKeyPath = filepath.Join(gitopsDir+"-secrets", "demo-key.txt")
				p.SSHKeyPath = filepath.Join(gitopsDir+"-secrets", "demo")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := valid
			tt.mutate(&p)
			err := p.Validate()
			if tt.name == "common prefix sibling stays valid" {
				if err != nil {
					t.Fatalf("Validate() error = %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("Validate() error = nil, want containment error")
			}
		})
	}
}

func TestClusterPathsValidateRejectsSymlinkedParentsIntoGitOps(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires elevated privileges on many Windows test hosts")
	}

	root := t.TempDir()
	gitopsDir := filepath.Join(root, "gitops", "acme")
	if err := os.MkdirAll(gitopsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	linkRoot := filepath.Join(root, "state-link")
	if err := os.Symlink(gitopsDir, linkRoot); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}

	p := ClusterPaths{
		GitOpsDir:       gitopsDir,
		ClusterStateDir: filepath.Join(linkRoot, "demo"),
		SecretsDir:      filepath.Join(root, "secrets", "acme", "demo"),
		ConfigPath:      filepath.Join(linkRoot, "demo", "demo-config.yaml"),
		SOPSKeyPath:     filepath.Join(root, "secrets", "acme", "demo", "age", "keys", "demo-key.txt"),
		SSHKeyPath:      filepath.Join(root, "secrets", "acme", "demo", "ssh", "demo"),
	}

	err := p.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want symlink containment error")
	}
	if !strings.Contains(err.Error(), "gitops") {
		t.Fatalf("Validate() error = %v, want gitops containment context", err)
	}
}

func TestResolveWithFallbackUsesOnlyNewStateLayout(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	clustersRoot := filepath.Join(root, "clusters")
	blueprintsRoot := filepath.Join(clustersRoot, "blueprints")
	gitopsRoot := filepath.Join(root, "gitops")
	stateRoot := filepath.Join(root, "state")
	secretsRoot := filepath.Join(root, "secrets")
	resolver := NewPathResolverWithRoots(clustersRoot, blueprintsRoot, gitopsRoot, stateRoot, secretsRoot, DefaultResolutionOptions())

	newBlueprintsDir := filepath.Join(blueprintsRoot, "org1", "demo")
	if err := os.MkdirAll(newBlueprintsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(newBlueprintsDir, "demo-config.yaml"), []byte("schema_version: \"2.0\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	legacyOrgDir := filepath.Join(clustersRoot, "legacy-org")
	if err := os.MkdirAll(filepath.Join(legacyOrgDir, "infrastructure", "clusters", "demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacyOrgDir, ".demo-config.yaml"), []byte("schema_version: \"2.0\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	clusterPaths, err := resolver.ResolveWithFallback(context.Background(), "demo")
	if err != nil {
		t.Fatalf("ResolveWithFallback() error = %v", err)
	}
	expectedClusterDir := filepath.Join(gitopsRoot, "org1", "infrastructure", "clusters", "demo")
	if clusterPaths.ClusterDir != expectedClusterDir {
		t.Fatalf("ClusterDir = %q, want %q", clusterPaths.ClusterDir, expectedClusterDir)
	}
	expectedKubeconfig := filepath.Join(secretsRoot, "org1", "demo", "kubernetes", "kubeconfig.yaml")
	if clusterPaths.KubeconfigPath != expectedKubeconfig {
		t.Fatalf("KubeconfigPath = %q, want %q", clusterPaths.KubeconfigPath, expectedKubeconfig)
	}

	ambiguousDir := filepath.Join(blueprintsRoot, "org2", "demo")
	if err := os.MkdirAll(ambiguousDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ambiguousDir, "demo-config.yaml"), []byte("schema_version: \"2.0\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	resolver.ClearCache()

	if _, err := resolver.ResolveWithFallback(context.Background(), "demo"); err == nil {
		t.Fatal("ResolveWithFallback() error = nil, want ambiguity error")
	}
}

// TestGitOpsRepoWithInfrastructureDirIsNotLegacy verifies that a git-initialized
// GitOps repository containing only infrastructure/clusters/<name> (as created
// by cluster init) is NOT flagged as a legacy layout. Regression test for a
// false-positive where the presence of infrastructure/clusters/<name> alone was
// incorrectly treated as a legacy marker.
func TestGitOpsRepoWithInfrastructureDirIsNotLegacy(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	clustersRoot := filepath.Join(root, "clusters")
	blueprintsRoot := filepath.Join(clustersRoot, "blueprints")
	gitopsRoot := filepath.Join(root, "gitops")
	stateRoot := filepath.Join(root, "state")
	secretsRoot := filepath.Join(root, "secrets")

	organization := "my-org"
	clusterName := "my-cluster"

	// Simulate what cluster init does: create the org directory inside
	// ClustersDir with .git and infrastructure/clusters/<name>.
	orgDir := filepath.Join(clustersRoot, organization)
	if err := os.MkdirAll(filepath.Join(orgDir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(orgDir, "infrastructure", "clusters", clusterName), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(orgDir, "applications", "overlays", clusterName), 0o755); err != nil {
		t.Fatal(err)
	}

	// Set up a valid blueprint so the cluster can be resolved.
	if err := os.MkdirAll(filepath.Join(blueprintsRoot, organization, clusterName), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(blueprintsRoot, organization, clusterName, clusterName+"-config.yaml"),
		[]byte("schema_version: \"2.0\"\n"), 0o600,
	); err != nil {
		t.Fatal(err)
	}

	resolver := NewPathResolverWithRoots(clustersRoot, blueprintsRoot, gitopsRoot, stateRoot, secretsRoot, DefaultResolutionOptions())

	// CanResolve must not return a LegacyLayoutError.
	strategy := NewOrgBasedStrategyWithRoots(resolver.GetRoots())
	canResolve, err := strategy.CanResolve(context.Background(), clusterName, organization)
	if err != nil {
		if _, ok := err.(*LegacyLayoutError); ok {
			t.Fatalf("CanResolve() returned LegacyLayoutError for a valid GitOps repo with only infrastructure dirs")
		}
		t.Fatalf("CanResolve() unexpected error: %v", err)
	}
	if !canResolve {
		t.Log("CanResolve() = false (blueprint not in gitops zone), which is acceptable")
	}

	// detectLegacyLayoutForCluster must not return a LegacyLayoutError.
	if err := resolver.detectLegacyLayoutForCluster(clusterName); err != nil {
		if _, ok := err.(*LegacyLayoutError); ok {
			t.Fatalf("detectLegacyLayoutForCluster() returned LegacyLayoutError for a valid GitOps repo with only infrastructure dirs")
		}
		t.Fatalf("detectLegacyLayoutForCluster() unexpected error: %v", err)
	}
}

// TestGitOpsRepoWithSecretsIsLegacy verifies that a git repo with a secrets/
// directory is still correctly detected as a legacy layout.
func TestGitOpsRepoWithSecretsIsLegacy(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	clustersRoot := filepath.Join(root, "clusters")
	blueprintsRoot := filepath.Join(clustersRoot, "blueprints")
	gitopsRoot := filepath.Join(root, "gitops")
	stateRoot := filepath.Join(root, "state")
	secretsRoot := filepath.Join(root, "secrets")

	organization := "legacy-org"
	clusterName := "legacy-cluster"

	// Create org directory with .git, infrastructure dirs, AND secrets (legacy).
	orgDir := filepath.Join(clustersRoot, organization)
	if err := os.MkdirAll(filepath.Join(orgDir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(orgDir, "infrastructure", "clusters", clusterName), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(orgDir, "secrets"), 0o755); err != nil {
		t.Fatal(err)
	}

	resolver := NewPathResolverWithRoots(clustersRoot, blueprintsRoot, gitopsRoot, stateRoot, secretsRoot, DefaultResolutionOptions())

	// detectLegacyLayoutForCluster MUST return a LegacyLayoutError.
	err := resolver.detectLegacyLayoutForCluster(clusterName)
	if err == nil {
		t.Fatal("detectLegacyLayoutForCluster() = nil, want LegacyLayoutError for layout with secrets dir")
	}
	if _, ok := err.(*LegacyLayoutError); !ok {
		t.Fatalf("detectLegacyLayoutForCluster() error = %T %v, want LegacyLayoutError", err, err)
	}

	// Strategy's rejectLegacyLayout must also flag it.
	strategy := NewOrgBasedStrategyWithRoots(resolver.GetRoots())
	_, err = strategy.CanResolve(context.Background(), clusterName, organization)
	if err == nil {
		t.Fatal("CanResolve() = nil error, want LegacyLayoutError for layout with secrets dir")
	}
	if _, ok := err.(*LegacyLayoutError); !ok {
		t.Fatalf("CanResolve() error = %T %v, want LegacyLayoutError", err, err)
	}
}

// TestGitOpsRepoWithDotConfigFileIsLegacy verifies that a git repo with a
// .<cluster>-config.yaml file is correctly detected as a legacy layout.
func TestGitOpsRepoWithDotConfigFileIsLegacy(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	clustersRoot := filepath.Join(root, "clusters")
	blueprintsRoot := filepath.Join(clustersRoot, "blueprints")
	gitopsRoot := filepath.Join(root, "gitops")
	stateRoot := filepath.Join(root, "state")
	secretsRoot := filepath.Join(root, "secrets")

	organization := "config-org"
	clusterName := "config-cluster"

	// Create org directory with .git and a dot-config file (legacy marker).
	orgDir := filepath.Join(clustersRoot, organization)
	if err := os.MkdirAll(filepath.Join(orgDir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(orgDir, "."+clusterName+"-config.yaml"),
		[]byte("schema_version: \"2.0\"\n"), 0o600,
	); err != nil {
		t.Fatal(err)
	}

	resolver := NewPathResolverWithRoots(clustersRoot, blueprintsRoot, gitopsRoot, stateRoot, secretsRoot, DefaultResolutionOptions())

	// detectLegacyLayoutForCluster MUST return a LegacyLayoutError.
	err := resolver.detectLegacyLayoutForCluster(clusterName)
	if err == nil {
		t.Fatal("detectLegacyLayoutForCluster() = nil, want LegacyLayoutError for layout with dot-config file")
	}
	if _, ok := err.(*LegacyLayoutError); !ok {
		t.Fatalf("detectLegacyLayoutForCluster() error = %T %v, want LegacyLayoutError", err, err)
	}

	// Strategy's rejectLegacyLayout must also flag it.
	strategy := NewOrgBasedStrategyWithRoots(resolver.GetRoots())
	_, err = strategy.CanResolve(context.Background(), clusterName, organization)
	if err == nil {
		t.Fatal("CanResolve() = nil error, want LegacyLayoutError for layout with dot-config file")
	}
	if _, ok := err.(*LegacyLayoutError); !ok {
		t.Fatalf("CanResolve() error = %T %v, want LegacyLayoutError", err, err)
	}
}

func TestResolveWithFallbackRejectsLegacyLayoutEvenWhenSecureStateExists(t *testing.T) {
	root := t.TempDir()
	clustersRoot := filepath.Join(root, "clusters")
	gitopsRoot := filepath.Join(root, "gitops")
	stateRoot := filepath.Join(root, "state")
	secretsRoot := filepath.Join(root, "secrets")
	resolver := NewPathResolverWithRoots(clustersRoot, filepath.Join(clustersRoot, "blueprints"), gitopsRoot, stateRoot, secretsRoot, DefaultResolutionOptions())

	stateDir := filepath.Join(stateRoot, "acme", "demo")
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, "demo-config.yaml"), []byte("schema_version: \"2.0\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	legacyOrgDir := filepath.Join(clustersRoot, "acme")
	if err := os.MkdirAll(filepath.Join(legacyOrgDir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacyOrgDir, ".demo-config.yaml"), []byte("schema_version: \"2.0\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := resolver.ResolveWithFallback(context.Background(), "demo"); err == nil {
		t.Fatal("ResolveWithFallback() error = nil, want legacy layout error")
	} else if _, ok := err.(*LegacyLayoutError); !ok {
		t.Fatalf("ResolveWithFallback() error = %T %v, want LegacyLayoutError", err, err)
	}

	if _, err := resolver.GetOrganization(context.Background(), "demo"); err == nil {
		t.Fatal("GetOrganization() error = nil, want legacy layout error")
	} else if _, ok := err.(*LegacyLayoutError); !ok {
		t.Fatalf("GetOrganization() error = %T %v, want LegacyLayoutError", err, err)
	}
}
