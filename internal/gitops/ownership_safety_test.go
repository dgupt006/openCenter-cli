package gitops

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPromoteOverlayAllowsSymlinkedAncestor(t *testing.T) {
	workspace := t.TempDir()
	base := t.TempDir()
	realParent := filepath.Join(base, "real")
	target := filepath.Join(realParent, "overlay")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	linkedParent := filepath.Join(base, "linked")
	if err := os.Symlink(realParent, linkedParent); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	writeTestFile(t, filepath.Join(workspace, "services", "metallb", "generated.yaml"), "generated")
	if _, err := promoteOverlay(workspace, filepath.Join(linkedParent, "overlay"), "cluster", PromoteOptions{}); err != nil {
		t.Fatalf("promote through symlinked ancestor: %v", err)
	}
	if data, err := os.ReadFile(filepath.Join(target, "services", "metallb", "generated.yaml")); err != nil || string(data) != "generated" {
		t.Fatalf("generated file = %q, %v", data, err)
	}
}

func TestPromoteOverlayRejectsSymlinkedTargetOverlay(t *testing.T) {
	workspace := t.TempDir()
	base := t.TempDir()
	realTarget := filepath.Join(base, "real-overlay")
	if err := os.MkdirAll(realTarget, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(base, "target-overlay")
	if err := os.Symlink(realTarget, target); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	writeTestFile(t, filepath.Join(workspace, "services", "metallb", "generated.yaml"), "generated")
	if _, err := promoteOverlay(workspace, target, "cluster", PromoteOptions{}); err == nil || !strings.Contains(err.Error(), "symlinked target overlay") {
		t.Fatalf("expected symlink refusal, got %v", err)
	}
}

func TestPromoteOverlayDoesNotPruneModifiedTrackedFile(t *testing.T) {
	workspace := t.TempDir()
	root := t.TempDir()
	path := filepath.Join(workspace, "services", "metallb", "generated.yaml")
	writeTestFile(t, path, "generated")
	if _, err := promoteOverlay(workspace, root, "cluster", PromoteOptions{}); err != nil {
		t.Fatal(err)
	}

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	tracked := filepath.Join(root, "services", "metallb", "generated.yaml")
	writeTestFile(t, tracked, "modified")
	_, err := promoteOverlay(workspace, root, "cluster", PromoteOptions{Force: true})
	if err == nil || !strings.Contains(err.Error(), "ownership conflict") {
		t.Fatalf("expected ownership conflict, got %v", err)
	}
	data, readErr := os.ReadFile(tracked)
	if readErr != nil || string(data) != "modified" {
		t.Fatalf("modified tracked file was deleted or changed: %q, %v", data, readErr)
	}
}

func TestPromoteOverlayPrunesMatchingTrackedFile(t *testing.T) {
	workspace := t.TempDir()
	root := t.TempDir()
	path := filepath.Join(workspace, "services", "metallb", "generated.yaml")
	writeTestFile(t, path, "generated")
	if _, err := promoteOverlay(workspace, root, "cluster", PromoteOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}

	result, err := promoteOverlay(workspace, root, "cluster", PromoteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Pruned) != 1 || result.Pruned[0] != "services/metallb/generated.yaml" {
		t.Fatalf("prune result = %v", result.Pruned)
	}
	if _, err := os.Stat(filepath.Join(root, "services", "metallb", "generated.yaml")); !os.IsNotExist(err) {
		t.Fatalf("matching tracked file still exists: %v", err)
	}
}

func TestLoadGeneratedManifestRejectsSymlink(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "manifest-target.json")
	writeTestFile(t, target, "{}")
	manifestPath := filepath.Join(root, GeneratedManifestFile)
	if err := os.Symlink(target, manifestPath); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, _, err := loadGeneratedManifest(root); err == nil || !strings.Contains(err.Error(), "symlinked generated manifest") {
		t.Fatalf("expected symlink refusal, got %v", err)
	}
}

func TestWriteGeneratedManifestRejectsSymlink(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "manifest-target.json")
	writeTestFile(t, target, "old")
	manifestPath := filepath.Join(root, GeneratedManifestFile)
	if err := os.Symlink(target, manifestPath); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	manifest := GeneratedManifest{Version: ownershipManifestVersion, Cluster: "cluster", Files: map[string]string{}}
	if err := writeGeneratedManifest(root, manifest); err == nil || !strings.Contains(err.Error(), "symlinked generated manifest") {
		t.Fatalf("expected symlink refusal, got %v", err)
	}
	data, err := os.ReadFile(target)
	if err != nil || string(data) != "old" {
		t.Fatalf("symlink target changed: %q, %v", data, err)
	}
}

func TestWriteGeneratedManifestAtomicallyReplacesFile(t *testing.T) {
	root := t.TempDir()
	old := GeneratedManifest{Version: ownershipManifestVersion, Cluster: "old", Files: map[string]string{}}
	if err := writeGeneratedManifest(root, old); err != nil {
		t.Fatal(err)
	}
	newManifest := GeneratedManifest{Version: ownershipManifestVersion, Cluster: "new", Files: map[string]string{"services/metallb/generated.yaml": "hash"}}
	if err := writeGeneratedManifest(root, newManifest); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(root, GeneratedManifestFile))
	if err != nil {
		t.Fatal(err)
	}
	var got GeneratedManifest
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Cluster != "new" || got.Files["services/metallb/generated.yaml"] != "hash" {
		t.Fatalf("replacement manifest = %+v", got)
	}
	matches, err := filepath.Glob(filepath.Join(root, ".opencenter-generated-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary manifest files remain: %v", matches)
	}
}
