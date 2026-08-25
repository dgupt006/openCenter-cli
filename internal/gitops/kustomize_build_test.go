package gitops

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
)

func TestRenderMetalLBOverlayBuildsWithKustomize(t *testing.T) {
	repo := t.TempDir()
	cfg := newDefault("kustomize-metallb")
	cfg.OpenCenter.GitOps.Repository.LocalDir = repo
	cfg.OpenCenter.Services = cloneServiceMap(cfg.OpenCenter.Services)
	cfg.OpenCenter.Services["metallb"] = &services.MetalLBConfig{
		BaseConfig: services.BaseConfig{Enabled: true, Namespace: "metallb-system", OverlayFilesRendererKey: "metallb"},
		IPAddressPools: []services.IPAddressPool{{
			Name:      "public-pool",
			Addresses: []string{"10.0.0.1/32"},
		}},
		L2Advertisements: []services.L2Advertisement{{
			Name:           "public-pool-l2",
			IPAddressPools: []string{"public-pool"},
		}},
	}
	if err := RenderClusterApps(cfg); err != nil {
		t.Fatalf("render: %v", err)
	}

	serviceRoot := filepath.Join(repo, "applications", "overlays", cfg.ClusterName(), "services", "metallb")
	output := runKustomizeBuild(t, serviceRoot)
	if !strings.Contains(output, "kind: IPAddressPool") || !strings.Contains(output, "kind: L2Advertisement") {
		t.Fatalf("kustomize output missing MetalLB resources:\n%s", output)
	}
	t.Logf("asserted kustomize output snippet:\n%s", kustomizeKinds(output, "IPAddressPool", "L2Advertisement"))
}

func TestServiceOverlayWithoutCustomResourcesBuilds(t *testing.T) {
	repo := t.TempDir()
	cfg := newDefault("kustomize-empty-custom")
	cfg.OpenCenter.GitOps.Repository.LocalDir = repo
	cfg.OpenCenter.Services = cloneServiceMap(cfg.OpenCenter.Services)
	cfg.OpenCenter.Services["metallb"] = &services.MetalLBConfig{BaseConfig: services.BaseConfig{Enabled: true, OverlayFilesRendererKey: "metallb"}}
	if err := RenderClusterApps(cfg); err != nil {
		t.Fatalf("render: %v", err)
	}

	serviceRoot := filepath.Join(repo, "applications", "overlays", cfg.ClusterName(), "services", "metallb")
	kustomization, err := os.ReadFile(filepath.Join(serviceRoot, "kustomization.yaml"))
	if err != nil {
		t.Fatalf("read service kustomization: %v", err)
	}
	if !strings.Contains(string(kustomization), "custom/") {
		t.Fatalf("service kustomization does not reference custom layer:\n%s", kustomization)
	}
	if _, err := os.Stat(filepath.Join(serviceRoot, "custom", "kustomization.yaml")); err != nil {
		t.Fatalf("custom layer seed missing: %v", err)
	}
	output := runKustomizeBuild(t, serviceRoot)
	t.Logf("kustomize output for service without custom resources:\n%s", output)
}

func runKustomizeBuild(t *testing.T, root string) string {
	t.Helper()
	var cmd *exec.Cmd
	if _, err := exec.LookPath("kustomize"); err == nil {
		cmd = exec.Command("kustomize", "build", root)
	} else if _, err := exec.LookPath("kubectl"); err == nil {
		cmd = exec.Command("kubectl", "kustomize", root)
	} else {
		t.Skip("kustomize build skipped: neither kustomize nor kubectl is installed")
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("kustomize build %s: %v\n%s", root, err, output)
	}
	return string(output)
}

func kustomizeKinds(output string, kinds ...string) string {
	lines := strings.Split(output, "\n")
	var snippet []string
	for i, line := range lines {
		for _, kind := range kinds {
			if line == "kind: "+kind {
				start := i - 1
				if start < 0 {
					start = 0
				}
				end := i + 1
				if end >= len(lines) {
					end = len(lines) - 1
				}
				snippet = append(snippet, strings.Join(lines[start:end+1], "\n"))
			}
		}
	}
	return strings.Join(snippet, "\n---\n")
}
