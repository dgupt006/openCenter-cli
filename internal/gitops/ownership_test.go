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
