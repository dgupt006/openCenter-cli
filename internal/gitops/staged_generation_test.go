package gitops

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/secretartifacts"
)

func TestCreateWorkspaceUsesPrivateGenerationRoots(t *testing.T) {
	manager := NewWorkspaceManager(t.TempDir())
	ctx := context.Background()
	workspace, err := manager.CreateWorkspace(ctx, newDefault("private-roots"))
	if err != nil {
		t.Fatalf("CreateWorkspace() error = %v", err)
	}
	defer manager.CleanupWorkspace(ctx, workspace)
	defer manager.(*DefaultWorkspaceManager).Shutdown(ctx)

	for name, path := range map[string]string{
		"workspace root": workspace.RootDir,
		"temporary root": workspace.TempDir,
	} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat %s: %v", name, err)
		}
		if got := info.Mode().Perm(); got != 0o700 {
			t.Errorf("%s permissions = %o, want 700", name, got)
		}
	}
}

func TestGenerateClusterTreeManifestFailureDoesNotMutateTarget(t *testing.T) {
	repo := t.TempDir()
	cfg := newDefault("staged-manifest-failure")
	cfg.OpenCenter.GitOps.Repository.LocalDir = repo
	target := filepath.Join(repo, "custom", "sentinel.txt")
	writeTestFile(t, target, "unchanged\n")
	before := snapshotStagedGenerationTree(t, repo)

	_, _, err := GenerateClusterTree(context.Background(), cfg, StagedGenerationOptions{
		IncludeInfrastructure: true,
		IncludeFluxBridge:     true,
		ValidateManifest: func(path string) error {
			if path == repo || strings.HasPrefix(path, repo+string(os.PathSeparator)) {
				t.Fatalf("manifest validator received live target %q", path)
			}
			return os.ErrInvalid
		},
		Promote: PromoteOptions{DryRun: true},
	})
	if err == nil || !strings.Contains(err.Error(), "manifest validation failed") {
		t.Fatalf("GenerateClusterTree() error = %v, want staged manifest failure", err)
	}
	if after := snapshotStagedGenerationTree(t, repo); !mapsEqual(after, before) {
		t.Fatalf("target changed after staged manifest failure:\n got=%v\nwant=%v", after, before)
	}
}

func TestGenerateClusterTreeOwnershipConflictDoesNotMutateTarget(t *testing.T) {
	repo := t.TempDir()
	cfg := newDefault("staged-ownership-conflict")
	cfg.OpenCenter.GitOps.Repository.LocalDir = repo
	baseOptions := StagedGenerationOptions{Promote: PromoteOptions{}}
	if _, _, err := GenerateClusterTree(context.Background(), cfg, baseOptions); err != nil {
		t.Fatalf("initial GenerateClusterTree() error = %v", err)
	}

	root := filepath.Join(repo, "applications", "overlays", cfg.ClusterName())
	writeTestFile(t, filepath.Join(root, "services", "metallb", "hand-authored.yaml"), "keep me\n")
	before := snapshotStagedGenerationTree(t, repo)
	_, _, err := GenerateClusterTree(context.Background(), cfg, baseOptions)
	if err == nil || !strings.Contains(err.Error(), "hand-authored.yaml") {
		t.Fatalf("GenerateClusterTree() error = %v, want ownership conflict", err)
	}
	if after := snapshotStagedGenerationTree(t, repo); !mapsEqual(after, before) {
		t.Fatalf("target changed after ownership refusal:\n got=%v\nwant=%v", after, before)
	}
}

func TestGenerateClusterTreeValidatesAndEncryptsSingleStagedTree(t *testing.T) {
	repo := t.TempDir()
	cfg := newDefault("single-staged-tree")
	cfg.OpenCenter.GitOps.Repository.LocalDir = repo
	var encryptCalls int
	var validatedPath string
	_, _, err := GenerateClusterTree(context.Background(), cfg, StagedGenerationOptions{
		Encrypt: func(_ context.Context, overlayPath string, _ *v2.Config) error {
			encryptCalls++
			if _, err := os.Stat(overlayPath); err != nil {
				return err
			}
			return nil
		},
		ValidateManifest: func(path string) error {
			validatedPath = path
			return nil
		},
		Promote: PromoteOptions{DryRun: true},
	})
	if err != nil {
		t.Fatalf("GenerateClusterTree() error = %v", err)
	}
	if encryptCalls != 1 {
		t.Fatalf("encrypt calls = %d, want 1", encryptCalls)
	}
	if validatedPath == "" || validatedPath == repo || strings.HasPrefix(validatedPath, repo+string(os.PathSeparator)) {
		t.Fatalf("manifest validation path = %q, want private staged root", validatedPath)
	}
}

func snapshotStagedGenerationTree(t *testing.T, root string) map[string]string {
	t.Helper()
	result := map[string]string{}
	if err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		result[filepath.ToSlash(rel)] = string(data)
		return nil
	}); err != nil {
		t.Fatalf("snapshot target: %v", err)
	}
	return result
}

func mapsEqual(left, right map[string]string) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		if right[key] != value {
			return false
		}
	}
	return true
}

func TestGenerateClusterTreeFinalizesBeforeValidationAndDryRunMatchesApply(t *testing.T) {
	repo := t.TempDir()
	cfg := newDefault("finalized-tree")
	cfg.OpenCenter.GitOps.Repository.LocalDir = repo
	before := snapshotStagedGenerationTree(t, repo)
	var order []string
	options := StagedGenerationOptions{
		Materialize: func(root string) error {
			order = append(order, "materialize")
			clusterDir := filepath.Join(root, "infrastructure", "clusters", cfg.ClusterName())
			if err := os.MkdirAll(clusterDir, 0o700); err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(clusterDir, "provider.tf"), []byte("provider-final\n"), 0o644); err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(clusterDir, "terraform.tfvars"), []byte("secret-final\n"), 0o600)
		},
		Encrypt: func(_ context.Context, overlay string, _ *v2.Config) error {
			order = append(order, "encrypt")
			return os.WriteFile(filepath.Join(overlay, "finalized.marker"), []byte("encrypted-final\n"), 0o600)
		},
		ValidateManifest: func(root string) error {
			order = append(order, "validate")
			for path, want := range map[string]string{
				filepath.Join(root, "applications", "overlays", cfg.ClusterName(), "finalized.marker"):   "encrypted-final\n",
				filepath.Join(root, "infrastructure", "clusters", cfg.ClusterName(), "provider.tf"):      "provider-final\n",
				filepath.Join(root, "infrastructure", "clusters", cfg.ClusterName(), "terraform.tfvars"): "secret-final\n",
			} {
				got, err := os.ReadFile(path)
				if err != nil || string(got) != want {
					return fmt.Errorf("finalized staged file %s = %q, %v", path, got, err)
				}
			}
			return nil
		},
		IncludeInfrastructure: true,
		IncludeFluxBridge:     true,
		Promote:               PromoteOptions{DryRun: true},
	}

	dry, _, err := GenerateClusterTree(context.Background(), cfg, options)
	if err != nil {
		t.Fatalf("dry-run GenerateClusterTree() error = %v", err)
	}
	if !reflect.DeepEqual(order, []string{"materialize", "encrypt", "validate"}) {
		t.Fatalf("finalization order = %v", order)
	}
	if after := snapshotStagedGenerationTree(t, repo); !mapsEqual(after, before) {
		t.Fatalf("dry-run mutated target: got=%v want=%v", after, before)
	}
	for _, path := range []string{
		"applications/overlays/finalized-tree/finalized.marker",
		"infrastructure/clusters/finalized-tree/provider.tf",
		"infrastructure/clusters/finalized-tree/terraform.tfvars",
	} {
		if !stagedContainsString(dry.Added, path) {
			t.Fatalf("dry-run classification missing finalized path %q: %+v", path, dry)
		}
	}

	order = nil
	options.Promote.DryRun = false
	applied, _, err := GenerateClusterTree(context.Background(), cfg, options)
	if err != nil {
		t.Fatalf("apply GenerateClusterTree() error = %v", err)
	}
	if !reflect.DeepEqual(order, []string{"materialize", "encrypt", "validate"}) {
		t.Fatalf("apply finalization order = %v", order)
	}
	if !samePromotionCategories(dry, applied) {
		t.Fatalf("dry-run/apply classification differs:\n dry=%+v\napply=%+v", dry, applied)
	}
	if info, err := os.Stat(filepath.Join(repo, "infrastructure", "clusters", cfg.ClusterName(), "terraform.tfvars")); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("promoted terraform.tfvars mode = %v, %v", info, err)
	}
}

func TestGenerateClusterTreeCompletePreflightRejectsModifiedTrackedBeforeBackup(t *testing.T) {
	repo := t.TempDir()
	cfg := newDefault("complete-preflight")
	cfg.OpenCenter.GitOps.Repository.LocalDir = repo
	options := StagedGenerationOptions{IncludeInfrastructure: true, IncludeFluxBridge: true}
	initial, _, err := GenerateClusterTree(context.Background(), cfg, options)
	if err != nil {
		t.Fatalf("initial GenerateClusterTree() error = %v", err)
	}
	trackedRel := ""
	for _, path := range initial.Added {
		if strings.HasPrefix(path, "infrastructure/") {
			trackedRel = path
			break
		}
	}
	if trackedRel == "" {
		t.Fatalf("initial promotion did not classify infrastructure files: %+v", initial)
	}
	tracked := filepath.Join(repo, filepath.FromSlash(trackedRel))
	writeTestFile(t, tracked, "manually modified\n")
	custom := filepath.Join(repo, "infrastructure", "clusters", cfg.ClusterName(), "custom", "user.tf")
	writeTestFile(t, custom, "custom\n")
	before := snapshotStagedGenerationTree(t, repo)
	backupCalls := 0
	options.BeforePromote = func() error {
		backupCalls++
		return nil
	}

	_, _, err = GenerateClusterTree(context.Background(), cfg, options)
	if err == nil || !strings.Contains(err.Error(), "modified tracked file") {
		t.Fatalf("GenerateClusterTree() error = %v, want complete-tree tracked conflict", err)
	}
	if backupCalls != 0 {
		t.Fatalf("backup callback calls = %d, want 0 before complete preflight", backupCalls)
	}
	if after := snapshotStagedGenerationTree(t, repo); !mapsEqual(after, before) {
		t.Fatalf("complete-tree conflict mutated target: got=%v want=%v", after, before)
	}
}

func TestGenerateClusterTreeRollbackRestoresCompleteLiveTree(t *testing.T) {
	repo := t.TempDir()
	cfg := newDefault("rollback-tree")
	cfg.OpenCenter.GitOps.Repository.LocalDir = repo
	materialize := func(content string) func(string) error {
		return func(root string) error {
			path := filepath.Join(root, "infrastructure", "clusters", cfg.ClusterName(), "provider.tf")
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				return err
			}
			return os.WriteFile(path, []byte(content), 0o644)
		}
	}
	if _, _, err := GenerateClusterTree(context.Background(), cfg, StagedGenerationOptions{IncludeInfrastructure: true, Materialize: materialize("v1\n")}); err != nil {
		t.Fatalf("initial GenerateClusterTree() error = %v", err)
	}
	before := snapshotStagedGenerationTree(t, repo)
	mutations := 0
	generatedTreeMutationHook = func(string) error {
		mutations++
		if mutations == 2 {
			return fmt.Errorf("synthetic mid-promotion failure")
		}
		return nil
	}
	defer func() { generatedTreeMutationHook = nil }()

	_, _, err := GenerateClusterTree(context.Background(), cfg, StagedGenerationOptions{IncludeInfrastructure: true, Materialize: materialize("v2\n")})
	if err == nil || !strings.Contains(err.Error(), "synthetic mid-promotion failure") {
		t.Fatalf("GenerateClusterTree() error = %v, want injected promotion failure", err)
	}
	if mutations < 2 {
		t.Fatalf("promotion mutations = %d, want mid-transaction failure", mutations)
	}
	if after := snapshotStagedGenerationTree(t, repo); !mapsEqual(after, before) {
		t.Fatalf("rollback left partial live state:\n got=%v\nwant=%v", after, before)
	}
}

func stagedContainsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func samePromotionCategories(left, right *PromoteResult) bool {
	return reflect.DeepEqual(left.Added, right.Added) &&
		reflect.DeepEqual(left.Updated, right.Updated) &&
		reflect.DeepEqual(left.Unchanged, right.Unchanged) &&
		reflect.DeepEqual(left.Pruned, right.Pruned) &&
		reflect.DeepEqual(left.PruneCandidates, right.PruneCandidates) &&
		reflect.DeepEqual(left.Seeded, right.Seeded) &&
		reflect.DeepEqual(left.Renamed, right.Renamed) &&
		reflect.DeepEqual(left.Adopted, right.Adopted)
}

func TestGenerateClusterTreeProtectsUnknownAndCustomFiles(t *testing.T) {
	repo := t.TempDir()
	cfg := newDefault("tree-user-files")
	cfg.OpenCenter.GitOps.Repository.LocalDir = repo
	options := StagedGenerationOptions{IncludeInfrastructure: true, IncludeFluxBridge: true}
	if _, _, err := GenerateClusterTree(context.Background(), cfg, options); err != nil {
		t.Fatalf("initial GenerateClusterTree() error = %v", err)
	}
	infraDir := filepath.Join(repo, "infrastructure", "clusters", cfg.ClusterName())
	custom := filepath.Join(infraDir, "custom", "user.tf")
	unknown := filepath.Join(infraDir, "hand-authored.tf")
	writeTestFile(t, custom, "custom\n")
	writeTestFile(t, unknown, "unknown\n")
	before := snapshotStagedGenerationTree(t, repo)

	_, _, err := GenerateClusterTree(context.Background(), cfg, options)
	if err == nil || !strings.Contains(err.Error(), "hand-authored.tf") {
		t.Fatalf("GenerateClusterTree() error = %v, want unknown complete-tree conflict", err)
	}
	if after := snapshotStagedGenerationTree(t, repo); !mapsEqual(after, before) {
		t.Fatalf("unknown-file refusal mutated target: got=%v want=%v", after, before)
	}
	if err := os.Remove(unknown); err != nil {
		t.Fatal(err)
	}
	runtimeState := filepath.Join(infraDir, ".terraform", "terraform.tfstate")
	writeTestFile(t, runtimeState, "runtime-state\n")
	if _, _, err := GenerateClusterTree(context.Background(), cfg, options); err != nil {
		t.Fatalf("regeneration with custom/runtime files error = %v", err)
	}
	if got, err := os.ReadFile(custom); err != nil || string(got) != "custom\n" {
		t.Fatalf("custom file changed: %q, %v", got, err)
	}
	if got, err := os.ReadFile(runtimeState); err != nil || string(got) != "runtime-state\n" {
		t.Fatalf("OpenTofu runtime state changed: %q, %v", got, err)
	}
}

// TestGenerateClusterTreeAllowsSecretsSyncedArtifactsOnRegenerate reproduces the
// generate -> secrets sync -> generate workflow. After a first generate, a
// `secrets sync` materializes per-service secret.yaml files and records them in
// the secret-artifacts ownership ledger (.opencenter-secret-artifacts.json). A
// second generate's complete-tree ownership preflight must NOT misclassify those
// secrets-sync-owned files as user-authored files in generator-owned paths.
// Regression guard for the missing secret-ledger consultation in planGeneratedTree.
func TestGenerateClusterTreeAllowsSecretsSyncedArtifactsOnRegenerate(t *testing.T) {
	repo, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("evalsymlinks: %v", err)
	}
	cfg := newDefault("regen-after-sync")
	cfg.OpenCenter.GitOps.Repository.LocalDir = repo
	options := StagedGenerationOptions{IncludeInfrastructure: true, IncludeFluxBridge: true}

	// First generate establishes the tree + tree manifest.
	if _, _, err := GenerateClusterTree(context.Background(), cfg, options); err != nil {
		t.Fatalf("initial GenerateClusterTree() error = %v", err)
	}

	// Simulate `secrets sync`: materialize a per-service secret.yaml under a
	// generator-owned namespace and record it in the secret-artifacts ledger.
	// (cert-manager is enabled by default and is a generator-owned service.)
	materializeSecretArtifacts(t, cfg, []secretartifacts.Artifact{
		{Path: "services/cert-manager/secret.yaml"},
	})

	// Second generate must succeed: the secret file is owned by secrets sync, not
	// user-authored, so the complete-tree preflight must not refuse.
	if _, _, err := GenerateClusterTree(context.Background(), cfg, options); err != nil {
		t.Fatalf("regenerate after secrets sync error = %v (secret.yaml misclassified as user-authored)", err)
	}

	// The secret file must still be present and untouched after regenerate.
	secretPath := filepath.Join(repo, "applications", "overlays", cfg.ClusterName(), "services", "cert-manager", "secret.yaml")
	if _, statErr := os.Stat(secretPath); statErr != nil {
		t.Fatalf("secrets-sync-owned secret.yaml must survive regenerate: %v", statErr)
	}
}
