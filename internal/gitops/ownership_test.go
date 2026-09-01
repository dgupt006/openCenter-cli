package gitops

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
)

func TestPromoteOverlayRoundTripRender(t *testing.T) {
	repo := t.TempDir()
	cfg := newDefault("ownership-round-trip")
	cfg.OpenCenter.GitOps.Repository.LocalDir = repo
	if err := RenderClusterApps(cfg); err != nil {
		t.Fatalf("first render: %v", err)
	}
	root := filepath.Join(repo, "applications", "overlays", cfg.ClusterName())
	before := snapshotFiles(t, root)
	if err := RenderClusterApps(cfg); err != nil {
		t.Fatalf("second render: %v", err)
	}
	if after := snapshotFiles(t, root); !reflect.DeepEqual(before, after) {
		t.Fatalf("second render changed files")
	}
}

func TestRenderClusterAppsSeedsCustomLayerForMetalLB(t *testing.T) {
	repo := t.TempDir()
	cfg := newDefault("ownership-custom-layer")
	cfg.OpenCenter.GitOps.Repository.LocalDir = repo
	cfg.OpenCenter.Services["metallb"].(*services.MetalLBConfig).Enabled = true

	if err := RenderClusterApps(cfg); err != nil {
		t.Fatalf("render: %v", err)
	}
	root := filepath.Join(repo, "applications", "overlays", cfg.ClusterName(), "services", "metallb")
	seed := filepath.Join(root, "custom", "kustomization.yaml")
	if _, err := os.Stat(seed); err != nil {
		t.Fatalf("seed missing: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(root, "kustomization.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "  - custom/") {
		t.Fatalf("service kustomization does not reference custom layer:\n%s", content)
	}
}

func TestRenderClusterAppsCustomLayerIsUserOwned(t *testing.T) {
	repo := t.TempDir()
	cfg := newDefault("ownership-custom-owned")
	cfg.OpenCenter.GitOps.Repository.LocalDir = repo
	cfg.OpenCenter.Services["metallb"].(*services.MetalLBConfig).Enabled = true
	if err := RenderClusterApps(cfg); err != nil {
		t.Fatalf("initial render: %v", err)
	}
	root := filepath.Join(repo, "applications", "overlays", cfg.ClusterName())
	seed := filepath.Join(root, "services", "metallb", "custom", "kustomization.yaml")
	userContent := "apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\nresources:\n  - my-pool.yaml\n"
	writeTestFile(t, seed, userContent)
	customFile := filepath.Join(root, "services", "metallb", "custom", "my-pool.yaml")
	writeTestFile(t, customFile, "kind: IPAddressPool\n")

	if err := RenderClusterApps(cfg); err != nil {
		t.Fatalf("regenerate: %v", err)
	}
	if err := RenderClusterApps(cfg); err != nil {
		t.Fatalf("regenerate second time: %v", err)
	}
	if content, err := os.ReadFile(seed); err != nil || string(content) != userContent {
		t.Fatalf("seed content changed: %q, %v", content, err)
	}
	if _, err := os.Stat(customFile); err != nil {
		t.Fatalf("custom file was pruned: %v", err)
	}
	manifest := readTestManifest(t, root)
	if _, found := manifest.Files["services/metallb/custom/kustomization.yaml"]; found {
		t.Fatal("custom seed was recorded in manifest")
	}
	if _, found := manifest.Files["services/metallb/custom/my-pool.yaml"]; found {
		t.Fatal("custom file was recorded in manifest")
	}
}

func TestPromoteOverlayReseedsDeletedCustomKustomization(t *testing.T) {
	workspace := t.TempDir()
	root := t.TempDir()
	seedPath := filepath.Join(workspace, "services", "metallb", "custom", "kustomization.yaml")
	seedContent := "seed"
	writeTestFile(t, seedPath, seedContent)
	result, err := promoteOverlay(workspace, root, "cluster", PromoteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(result.Seeded, []string{"services/metallb/custom/kustomization.yaml"}) {
		t.Fatalf("seed result = %v", result.Seeded)
	}
	if err := os.Remove(filepath.Join(root, "services", "metallb", "custom", "kustomization.yaml")); err != nil {
		t.Fatal(err)
	}
	result, err = promoteOverlay(workspace, root, "cluster", PromoteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(result.Seeded, []string{"services/metallb/custom/kustomization.yaml"}) {
		t.Fatalf("reseed result = %v", result.Seeded)
	}
	content, err := os.ReadFile(filepath.Join(root, "services", "metallb", "custom", "kustomization.yaml"))
	if err != nil || string(content) != seedContent {
		t.Fatalf("reseed content = %q, %v", content, err)
	}
}

func TestPromoteOverlayExistingSeedIsNotAddedOrOverwritten(t *testing.T) {
	workspace := t.TempDir()
	root := t.TempDir()
	seed := filepath.Join(workspace, "services", "metallb", "custom", "kustomization.yaml")
	writeTestFile(t, seed, "generated")
	if _, err := promoteOverlay(workspace, root, "cluster", PromoteOptions{}); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, "services", "metallb", "custom", "kustomization.yaml"), "user-owned")
	result, err := promoteOverlay(workspace, root, "cluster", PromoteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Seeded) != 0 || len(result.Added) != 0 {
		t.Fatalf("existing seed reported as changed: seeded=%v added=%v", result.Seeded, result.Added)
	}
	content, err := os.ReadFile(filepath.Join(root, "services", "metallb", "custom", "kustomization.yaml"))
	if err != nil || string(content) != "user-owned" {
		t.Fatalf("existing seed changed: %q, %v", content, err)
	}
}

func TestRenderClusterAppsVerbatimKustomizationDoesNotGainCustomLayer(t *testing.T) {
	repo := t.TempDir()
	cfg := newDefault("ownership-verbatim-kustomization")
	cfg.OpenCenter.GitOps.Repository.LocalDir = repo
	if err := RenderClusterApps(cfg); err != nil {
		t.Fatalf("render: %v", err)
	}
	root := filepath.Join(repo, "applications", "overlays", cfg.ClusterName(), "services", "gateway")
	content, err := os.ReadFile(filepath.Join(root, "kustomization.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(content), "custom/") {
		t.Fatalf("verbatim kustomization gained custom layer:\n%s", content)
	}
}

func TestPromoteOverlayPrunesDisabledPlannedFile(t *testing.T) {
	repo := t.TempDir()
	cfg := newDefault("ownership-prune")
	cfg.OpenCenter.GitOps.Repository.LocalDir = repo
	if err := RenderClusterApps(cfg); err != nil {
		t.Fatalf("initial render: %v", err)
	}
	stale := filepath.Join(repo, "applications", "overlays", cfg.ClusterName(), "services", "cert-manager")
	if _, err := os.Stat(stale); err != nil {
		t.Fatalf("expected enabled service output: %v", err)
	}
	cfg.OpenCenter.Services["cert-manager"] = &services.CertManagerConfig{BaseConfig: services.BaseConfig{Enabled: false}}
	if err := RenderClusterApps(cfg); err != nil {
		t.Fatalf("regenerate with disabled service: %v", err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("disabled service output still exists: %v", err)
	}
}

func TestPromoteOverlayUnknownFileFailsWithoutMutation(t *testing.T) {
	root := t.TempDir()
	workspace := t.TempDir()
	writeTestFile(t, filepath.Join(workspace, "services", "metallb", "generated.yaml"), "generated")
	if _, err := promoteOverlay(workspace, root, "cluster", PromoteOptions{}); err != nil {
		t.Fatal(err)
	}
	unknown := filepath.Join(root, "services", "metallb", "hand-authored.yaml")
	writeTestFile(t, unknown, "keep")
	writeTestFile(t, filepath.Join(workspace, "services", "metallb", "generated.yaml"), "changed")
	_, err := promoteOverlay(workspace, root, "cluster", PromoteOptions{})
	if err == nil || !strings.Contains(err.Error(), "hand-authored.yaml") {
		t.Fatalf("expected unknown-file error, got %v", err)
	}
	data, readErr := os.ReadFile(unknown)
	if readErr != nil || string(data) != "keep" {
		t.Fatalf("unknown file changed or disappeared: %q, %v", data, readErr)
	}
}

func TestPromoteOverlayAdoptsPlannedFileMissingFromManifest(t *testing.T) {
	root := t.TempDir()
	workspace := t.TempDir()
	path := "services/metallb/generated.yaml"
	writeTestFile(t, filepath.Join(workspace, path), "generated")
	if _, err := promoteOverlay(workspace, root, "cluster", PromoteOptions{}); err != nil {
		t.Fatal(err)
	}

	manifest := readTestManifest(t, root)
	delete(manifest.Files, path)
	if err := writeGeneratedManifest(root, manifest); err != nil {
		t.Fatal(err)
	}
	result, err := promoteOverlay(workspace, root, "cluster", PromoteOptions{})
	if err != nil {
		t.Fatalf("adopt identical planned file: %v", err)
	}
	if !reflect.DeepEqual(result.Unchanged, []string{path}) {
		t.Fatalf("unchanged result = %v", result.Unchanged)
	}
	if result.Warnings != nil && strings.Contains(strings.Join(result.Warnings, "\n"), path) {
		t.Fatalf("adoption was not silent: %v", result.Warnings)
	}
	manifest = readTestManifest(t, root)
	if got := manifest.Files[path]; got != hashBytes([]byte("generated")) {
		t.Fatalf("adopted manifest hash = %q", got)
	}
}

func TestPromoteOverlayRejectsDifferingPlannedFileMissingFromManifest(t *testing.T) {
	root := t.TempDir()
	workspace := t.TempDir()
	path := "services/metallb/generated.yaml"
	writeTestFile(t, filepath.Join(workspace, path), "generated")
	if _, err := promoteOverlay(workspace, root, "cluster", PromoteOptions{}); err != nil {
		t.Fatal(err)
	}

	manifestBefore := readTestManifest(t, root)
	delete(manifestBefore.Files, path)
	if err := writeGeneratedManifest(root, manifestBefore); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, path), "hand-authored")

	_, err := promoteOverlay(workspace, root, "cluster", PromoteOptions{})
	if err == nil || !strings.Contains(err.Error(), "untracked planned file") {
		t.Fatalf("expected planned-file ownership conflict, got %v", err)
	}
	data, readErr := os.ReadFile(filepath.Join(root, path))
	if readErr != nil || string(data) != "hand-authored" {
		t.Fatalf("differing planned file changed: %q, %v", data, readErr)
	}
	manifestAfter := readTestManifest(t, root)
	if !reflect.DeepEqual(manifestAfter, manifestBefore) {
		t.Fatalf("manifest changed after rejected adoption: before=%+v after=%+v", manifestBefore, manifestAfter)
	}
}

func TestPromoteOverlayCustomFileIsUntouched(t *testing.T) {
	root := t.TempDir()
	workspace := t.TempDir()
	writeTestFile(t, filepath.Join(workspace, "services", "metallb", "generated.yaml"), "generated")
	if _, err := promoteOverlay(workspace, root, "cluster", PromoteOptions{}); err != nil {
		t.Fatal(err)
	}
	custom := filepath.Join(root, "services", "metallb", "custom", "hand-authored.yaml")
	writeTestFile(t, custom, "keep")
	if _, err := promoteOverlay(workspace, root, "cluster", PromoteOptions{}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(custom)
	if err != nil || string(data) != "keep" {
		t.Fatalf("custom file changed: %q, %v", data, err)
	}
	manifest := readTestManifest(t, root)
	if _, found := manifest.Files["services/metallb/custom/hand-authored.yaml"]; found {
		t.Fatal("custom file was recorded in manifest")
	}
}

func TestPromoteOverlayBootstrapBacksUpDifferingPlannedFile(t *testing.T) {
	repo := t.TempDir()
	root := filepath.Join(repo, "applications", "overlays", "cluster")
	workspace := t.TempDir()
	path := filepath.Join(root, "services", "metallb", "manifest.yaml")
	writeTestFile(t, path, "hand-authored")
	writeTestFile(t, filepath.Join(workspace, "services", "metallb", "manifest.yaml"), "generated")
	result, err := promoteOverlay(workspace, root, "cluster", PromoteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Warnings) == 0 {
		t.Fatal("expected bootstrap adoption warning")
	}
	matches, err := filepath.Glob(filepath.Join(repo, ".opencenter-backup", "*", "services", "metallb", "manifest.yaml"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("backup not created: %v, %v", matches, err)
	}
	data, err := os.ReadFile(matches[0])
	if err != nil || string(data) != "hand-authored" {
		t.Fatalf("backup content = %q, %v", data, err)
	}
}

func TestPromoteOverlayScopedMergeDoesNotPruneOtherService(t *testing.T) {
	root := t.TempDir()
	workspace := t.TempDir()
	writeTestFile(t, filepath.Join(workspace, "services", "one", "one.yaml"), "one")
	writeTestFile(t, filepath.Join(workspace, "services", "two", "two.yaml"), "two")
	if _, err := promoteOverlay(workspace, root, "cluster", PromoteOptions{}); err != nil {
		t.Fatal(err)
	}
	os.Remove(filepath.Join(workspace, "services", "two", "two.yaml"))
	result, err := promoteOverlay(workspace, root, "cluster", PromoteOptions{Scope: []string{"services/one"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Pruned) != 0 {
		t.Fatalf("scoped promote pruned unrelated files: %v", result.Pruned)
	}
	if _, err := os.Stat(filepath.Join(root, "services", "two", "two.yaml")); err != nil {
		t.Fatalf("unrelated service removed: %v", err)
	}
	manifest := readTestManifest(t, root)
	if _, found := manifest.Files["services/two/two.yaml"]; !found {
		t.Fatal("scoped promote dropped unrelated manifest entry")
	}
}

func TestPromoteOverlayScopeIgnoresUnrelatedOwnershipConflicts(t *testing.T) {
	root := t.TempDir()
	workspace := t.TempDir()
	onePath := "services/one/one.yaml"
	twoPath := "services/two/two.yaml"
	writeTestFile(t, filepath.Join(workspace, onePath), "one")
	writeTestFile(t, filepath.Join(workspace, twoPath), "two")
	if _, err := promoteOverlay(workspace, root, "cluster", PromoteOptions{}); err != nil {
		t.Fatal(err)
	}

	writeTestFile(t, filepath.Join(root, twoPath), "modified")
	if err := os.Remove(filepath.Join(workspace, twoPath)); err != nil {
		t.Fatal(err)
	}
	unrelatedUnknown := filepath.Join(root, "services", "two", "unknown.yaml")
	writeTestFile(t, unrelatedUnknown, "unrelated")
	result, err := promoteOverlay(workspace, root, "cluster", PromoteOptions{
		Scope:          []string{"services/one"},
		AdoptGenerated: true,
	})
	if err != nil {
		t.Fatalf("scoped promote rejected unrelated conflicts: %v", err)
	}
	if len(result.Updated) != 0 || len(result.Pruned) != 0 {
		t.Fatalf("scoped promote classified unrelated files: %+v", result)
	}
	if got, readErr := os.ReadFile(filepath.Join(root, twoPath)); readErr != nil || string(got) != "modified" {
		t.Fatalf("unrelated modified file changed: %q, %v", got, readErr)
	}
	if _, readErr := os.Stat(unrelatedUnknown); readErr != nil {
		t.Fatalf("unrelated unknown file changed: %v", readErr)
	}

	inScopeUnknown := filepath.Join(root, "services", "one", "unknown.yaml")
	writeTestFile(t, inScopeUnknown, "in-scope")
	if _, err := promoteOverlay(workspace, root, "cluster", PromoteOptions{Scope: []string{"services/one"}}); err == nil || !strings.Contains(err.Error(), "services/one/unknown.yaml") {
		t.Fatalf("expected in-scope unknown-file protection, got %v", err)
	}
}

func TestNormalizeOwnershipPathRejectsTraversal(t *testing.T) {
	if _, err := normalizeOwnershipPath("../../escape.yaml"); err == nil {
		t.Fatal("expected traversal path to be rejected")
	}
}

func TestPromoteOverlayCorruptManifestDoesNotMutate(t *testing.T) {
	root := t.TempDir()
	workspace := t.TempDir()
	writeTestFile(t, filepath.Join(root, GeneratedManifestFile), "{")
	writeTestFile(t, filepath.Join(workspace, "services", "metallb", "manifest.yaml"), "generated")
	if _, err := promoteOverlay(workspace, root, "cluster", PromoteOptions{}); err == nil || !strings.Contains(err.Error(), "corrupt JSON") {
		t.Fatalf("expected corrupt manifest error, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "services")); !os.IsNotExist(err) {
		t.Fatal("corrupt manifest caused target mutation")
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readTestManifest(t *testing.T, root string) GeneratedManifest {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, GeneratedManifestFile))
	if err != nil {
		t.Fatal(err)
	}
	var manifest GeneratedManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	return manifest
}

func snapshotFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	result := map[string]string{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
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
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestPromoteOverlayNoPruneReportsCandidatesAndRetainsManifest(t *testing.T) {
	root := t.TempDir()
	workspace := t.TempDir()
	path := "services/metallb/stale.yaml"
	writeTestFile(t, filepath.Join(workspace, path), "generated")
	if _, err := promoteOverlay(workspace, root, "cluster", PromoteOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(workspace, path)); err != nil {
		t.Fatal(err)
	}
	prune := false
	result, err := promoteOverlay(workspace, root, "cluster", PromoteOptions{Prune: &prune})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Pruned) != 0 {
		t.Fatalf("no-prune classified retained file as pruned: %v", result.Pruned)
	}
	if !reflect.DeepEqual(result.PruneCandidates, []string{path}) {
		t.Fatalf("prune candidates = %v", result.PruneCandidates)
	}
	if _, err := os.Stat(filepath.Join(root, path)); err != nil {
		t.Fatalf("no-prune removed candidate: %v", err)
	}
	if _, ok := readTestManifest(t, root).Files[path]; !ok {
		t.Fatal("no-prune dropped manifest ownership entry")
	}
	if !strings.Contains(strings.Join(result.Warnings, "\n"), "prune disabled") {
		t.Fatalf("no-prune result lacks retention warning: %v", result.Warnings)
	}
}

func TestPromoteOverlayDetectsUniqueSafeRename(t *testing.T) {
	root := t.TempDir()
	workspace := t.TempDir()
	oldPath := "services/one/old.yaml"
	newPath := "services/one/new.yaml"
	writeTestFile(t, filepath.Join(workspace, oldPath), "same")
	if _, err := promoteOverlay(workspace, root, "cluster", PromoteOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(workspace, oldPath)); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(workspace, newPath), "same")
	result, err := promoteOverlay(workspace, root, "cluster", PromoteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(result.Renamed, []string{oldPath + " -> " + newPath}) {
		t.Fatalf("renames = %v", result.Renamed)
	}
	if len(result.Added) != 0 || len(result.Pruned) != 0 {
		t.Fatalf("safe rename was classified as add+prune: added=%v pruned=%v", result.Added, result.Pruned)
	}
	if _, err := os.Stat(filepath.Join(root, oldPath)); !os.IsNotExist(err) {
		t.Fatalf("old path still exists: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, newPath)); err != nil {
		t.Fatalf("new path missing: %v", err)
	}
	manifest := readTestManifest(t, root)
	if _, ok := manifest.Files[oldPath]; ok {
		t.Fatal("old rename path remains in manifest")
	}
	if _, ok := manifest.Files[newPath]; !ok {
		t.Fatal("new rename path missing from manifest")
	}
}

func TestPromoteOverlayAmbiguousRenameRemainsAddAndPrune(t *testing.T) {
	root := t.TempDir()
	workspace := t.TempDir()
	oldPath := "services/one/old.yaml"
	newOne := "services/one/new-one.yaml"
	newTwo := "services/one/new-two.yaml"
	writeTestFile(t, filepath.Join(workspace, oldPath), "same")
	if _, err := promoteOverlay(workspace, root, "cluster", PromoteOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(workspace, oldPath)); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(workspace, newOne), "same")
	writeTestFile(t, filepath.Join(workspace, newTwo), "same")
	result, err := promoteOverlay(workspace, root, "cluster", PromoteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Renamed) != 0 {
		t.Fatalf("ambiguous rename classified as safe: %v", result.Renamed)
	}
	if !reflect.DeepEqual(result.Pruned, []string{oldPath}) {
		t.Fatalf("pruned = %v", result.Pruned)
	}
	if !reflect.DeepEqual(result.Added, []string{newOne, newTwo}) {
		t.Fatalf("added = %v", result.Added)
	}
}

func TestPromoteOverlayAdoptGeneratedBacksUpDifferingPlannedCollision(t *testing.T) {
	repo := t.TempDir()
	root := filepath.Join(repo, "applications", "overlays", "cluster")
	workspace := t.TempDir()
	path := "services/one/generated.yaml"
	writeTestFile(t, filepath.Join(workspace, path), "generated")
	if _, err := promoteOverlay(workspace, root, "cluster", PromoteOptions{}); err != nil {
		t.Fatal(err)
	}
	manifest := readTestManifest(t, root)
	delete(manifest.Files, path)
	if err := writeGeneratedManifest(root, manifest); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, path), "hand-authored")
	result, err := promoteOverlay(workspace, root, "cluster", PromoteOptions{AdoptGenerated: true})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(result.Adopted, []string{path}) {
		t.Fatalf("adopted = %v", result.Adopted)
	}
	if len(result.BackupPaths) != 1 {
		t.Fatalf("backup paths = %v", result.BackupPaths)
	}
	backup, err := os.ReadFile(result.BackupPaths[0])
	if err != nil || string(backup) != "hand-authored" {
		t.Fatalf("backup = %q, %v", backup, err)
	}
	if got, err := os.ReadFile(filepath.Join(root, path)); err != nil || string(got) != "generated" {
		t.Fatalf("adopted content = %q, %v", got, err)
	}
}

func TestPromoteOverlayDryRunAndApplyCategoriesMatch(t *testing.T) {
	root := t.TempDir()
	workspace := t.TempDir()
	writeTestFile(t, filepath.Join(workspace, "services", "one", "generated.yaml"), "generated")
	prune := true
	dry, err := promoteOverlay(workspace, root, "cluster", PromoteOptions{Prune: &prune, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	apply, err := promoteOverlay(workspace, root, "cluster", PromoteOptions{Prune: &prune})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(dry.Added, apply.Added) || !reflect.DeepEqual(dry.Updated, apply.Updated) || !reflect.DeepEqual(dry.Unchanged, apply.Unchanged) || !reflect.DeepEqual(dry.Pruned, apply.Pruned) || !reflect.DeepEqual(dry.PruneCandidates, apply.PruneCandidates) || !reflect.DeepEqual(dry.Renamed, apply.Renamed) || !reflect.DeepEqual(dry.Adopted, apply.Adopted) {
		t.Fatalf("dry-run/apply categories differ: dry=%+v apply=%+v", dry, apply)
	}
}

func TestPromoteOverlayDefaultRejectsModifiedTrackedFile(t *testing.T) {
	root := t.TempDir()
	workspace := t.TempDir()
	path := "services/one/generated.yaml"
	writeTestFile(t, filepath.Join(workspace, path), "generated")
	if _, err := promoteOverlay(workspace, root, "cluster", PromoteOptions{}); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, path), "modified")
	writeTestFile(t, filepath.Join(workspace, path), "new-generated")

	_, err := promoteOverlay(workspace, root, "cluster", PromoteOptions{})
	if err == nil || !strings.Contains(err.Error(), "modified tracked file") {
		t.Fatalf("expected default modified tracked protection, got %v", err)
	}
	if got, readErr := os.ReadFile(filepath.Join(root, path)); readErr != nil || string(got) != "modified" {
		t.Fatalf("modified tracked content changed: %q, %v", got, readErr)
	}
}

func TestPromoteOverlayAdoptGeneratedDoesNotOverwriteModifiedTrackedFile(t *testing.T) {
	root := t.TempDir()
	workspace := t.TempDir()
	path := "services/one/generated.yaml"
	writeTestFile(t, filepath.Join(workspace, path), "generated")
	if _, err := promoteOverlay(workspace, root, "cluster", PromoteOptions{}); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, path), "modified")
	writeTestFile(t, filepath.Join(workspace, path), "new-generated")
	_, err := promoteOverlay(workspace, root, "cluster", PromoteOptions{AdoptGenerated: true})
	if err == nil || !strings.Contains(err.Error(), "modified tracked file") {
		t.Fatalf("expected modified tracked protection, got %v", err)
	}
	if got, readErr := os.ReadFile(filepath.Join(root, path)); readErr != nil || string(got) != "modified" {
		t.Fatalf("modified tracked content changed: %q, %v", got, readErr)
	}
}

func TestPromoteOverlayAmbiguousStaleRenameRemainsAddAndPrune(t *testing.T) {
	root := t.TempDir()
	workspace := t.TempDir()
	oldOne := "services/one/old-one.yaml"
	oldTwo := "services/one/old-two.yaml"
	newPath := "services/one/new.yaml"
	writeTestFile(t, filepath.Join(workspace, oldOne), "same")
	writeTestFile(t, filepath.Join(workspace, oldTwo), "same")
	if _, err := promoteOverlay(workspace, root, "cluster", PromoteOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(workspace, oldOne)); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(workspace, oldTwo)); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(workspace, newPath), "same")
	result, err := promoteOverlay(workspace, root, "cluster", PromoteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Renamed) != 0 || !reflect.DeepEqual(result.Added, []string{newPath}) || !reflect.DeepEqual(result.Pruned, []string{oldOne, oldTwo}) {
		t.Fatalf("ambiguous stale rename classification: %+v", result)
	}
}

func TestPromoteOverlayAdoptGeneratedDoesNotAffectUnplannedOrCustomFiles(t *testing.T) {
	root := t.TempDir()
	workspace := t.TempDir()
	plannedPath := "services/one/generated.yaml"
	writeTestFile(t, filepath.Join(workspace, plannedPath), "generated")
	if _, err := promoteOverlay(workspace, root, "cluster", PromoteOptions{}); err != nil {
		t.Fatal(err)
	}
	unplannedPath := "services/one/unknown.yaml"
	customPath := "services/one/custom/user.yaml"
	writeTestFile(t, filepath.Join(root, unplannedPath), "unknown")
	writeTestFile(t, filepath.Join(root, customPath), "custom")

	_, err := promoteOverlay(workspace, root, "cluster", PromoteOptions{AdoptGenerated: true})
	if err == nil || !strings.Contains(err.Error(), "user-authored files") {
		t.Fatalf("expected unplanned-file refusal, got %v", err)
	}
	for path, want := range map[string]string{unplannedPath: "unknown", customPath: "custom"} {
		got, readErr := os.ReadFile(filepath.Join(root, path))
		if readErr != nil || string(got) != want {
			t.Fatalf("protected file %s changed: %q, %v", path, got, readErr)
		}
	}
}
