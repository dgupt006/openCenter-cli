package gitops

import (
	"path/filepath"
	"strings"
	"testing"

	configservices "github.com/opencenter-cloud/opencenter-cli/internal/config/services"
)

func TestOCTR651ObservabilitySourcesStageForLokiAndTempoOnly(t *testing.T) {
	dst := t.TempDir()
	cfg := newDefault("octr651-observability")
	cfg.OpenCenter.GitOps.Repository.LocalDir = dst

	cfg.OpenCenter.Services["kube-prometheus-stack"].(*configservices.PrometheusStackConfig).Enabled = false
	cfg.OpenCenter.Services["loki"].(*configservices.LokiConfig).Enabled = true
	cfg.OpenCenter.Services["tempo"].(*configservices.TempoConfig).Enabled = true

	if err := RenderClusterApps(cfg); err != nil {
		t.Fatalf("RenderClusterApps() error = %v", err)
	}

	fluxDir := filepath.Join(dst, "applications", "overlays", cfg.ClusterName(), "services", "fluxcd")
	aggregate := mustReadFile(t, filepath.Join(fluxDir, "kustomization.yaml"))
	for _, resource := range []string{"./observability-namespace.yaml", "./observability-sources.yaml"} {
		if !containsLine(aggregate, resource) {
			t.Errorf("services/fluxcd/kustomization.yaml missing %q:\n%s", resource, aggregate)
		}
	}

	sourcesPath := filepath.Join(fluxDir, "observability-sources.yaml")
	sourcesDocs, err := decodeYAMLDocuments([]byte(mustReadFile(t, sourcesPath)))
	if err != nil {
		t.Fatalf("parse %s: %v", sourcesPath, err)
	}
	sources := findFluxKustomization(t, sourcesDocs, "observability-sources")
	if got := nestedString(sources, "spec", "sourceRef", "name"); got != "opencenter-observability" {
		t.Errorf("observability-sources sourceRef.name = %q, want opencenter-observability", got)
	}
	if got := nestedString(sources, "spec", "path"); got != "applications/base/services/observability/sources" {
		t.Errorf("observability-sources spec.path = %q, want applications/base/services/observability/sources", got)
	}
	assertFluxDependencies(t, sources, "observability-sources", "sources", "observability-namespace")

	for _, service := range []string{"loki", "tempo"} {
		path := filepath.Join(fluxDir, service+".yaml")
		docs, err := decodeYAMLDocuments([]byte(mustReadFile(t, path)))
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		base := findFluxKustomization(t, docs, service+"-base")
		assertFluxDependencies(t, base, service+"-base", "observability-sources")
	}
}

func containsLine(content, want string) bool {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "- "))
		if line == want {
			return true
		}
	}
	return false
}

func findFluxKustomization(t *testing.T, docs []map[string]any, name string) map[string]any {
	t.Helper()
	for _, doc := range docs {
		if doc["kind"] != "Kustomization" {
			continue
		}
		if nestedString(doc, "metadata", "name") == name {
			return doc
		}
	}
	t.Fatalf("Flux Kustomization %q not found", name)
	return nil
}

func assertFluxDependencies(t *testing.T, doc map[string]any, stage string, wanted ...string) {
	t.Helper()
	dependencies, ok := nestedValue(doc, "spec", "dependsOn").([]any)
	if !ok {
		t.Fatalf("%s has no spec.dependsOn list: %#v", stage, doc)
	}
	got := make(map[string]bool, len(dependencies))
	for _, raw := range dependencies {
		dependency, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("%s has malformed dependency: %#v", stage, raw)
		}
		name, ok := dependency["name"].(string)
		if !ok {
			t.Fatalf("%s has dependency without a name: %#v", stage, dependency)
		}
		got[name] = true
	}
	for _, dependency := range wanted {
		if !got[dependency] {
			t.Errorf("%s must depend on %q, got %v", stage, dependency, got)
		}
	}
}

func nestedString(doc map[string]any, path ...string) string {
	value, _ := nestedValue(doc, path...).(string)
	return value
}

func nestedValue(doc map[string]any, path ...string) any {
	var value any = doc
	for _, key := range path {
		object, ok := value.(map[string]any)
		if !ok {
			return nil
		}
		value = object[key]
	}
	return value
}
