package gitops

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOCTR656KyvernoPolicyStagesUseDedicatedFluxKustomizations(t *testing.T) {
	cfg := newDefaultServiceConfig(t)

	actions, err := planClusterAppActions(cfg)
	require.NoError(t, err)

	var kyvernoContent string
	for _, action := range actions {
		if action.Output == "services/fluxcd/kyverno.yaml" {
			kyvernoContent = action.Content
		}
		require.NotEqual(t, "services/kyverno/kustomization.yaml", action.Output,
			"Kyverno must not create a customer-repository service kustomization")
	}
	require.NotEmpty(t, kyvernoContent, "planner did not produce services/fluxcd/kyverno.yaml")

	docs, err := decodeYAMLDocuments([]byte(kyvernoContent))
	require.NoError(t, err)
	require.Len(t, docs, 2, "Kyverno must render exactly base and default-ruleset Flux Kustomizations")

	byName := make(map[string]map[string]any, len(docs))
	for _, doc := range docs {
		meta, ok := doc["metadata"].(map[string]any)
		require.True(t, ok, "Flux Kustomization document has no metadata")
		name, ok := meta["name"].(string)
		require.True(t, ok, "Flux Kustomization metadata.name is not a string")
		byName[name] = doc
	}

	base := octr656RequireKustomizationDocument(t, byName, "kyverno-base")
	require.Equal(t, "applications/base/services/kyverno/policy-engine", octr656NestedString(base, "spec", "path"))
	require.Equal(t, "opencenter-kyverno", octr656NestedString(base, "spec", "sourceRef", "name"))

	ruleset := octr656RequireKustomizationDocument(t, byName, "kyverno-default-ruleset")
	require.Equal(t, "applications/base/services/kyverno/default-ruleset", octr656NestedString(ruleset, "spec", "path"))
	require.Equal(t, "opencenter-kyverno", octr656NestedString(ruleset, "spec", "sourceRef", "name"))
	require.ElementsMatch(t, []string{"sources", "kyverno-base"}, octr656DependencyNames(ruleset))
}

func octr656RequireKustomizationDocument(t *testing.T, docs map[string]map[string]any, name string) map[string]any {
	t.Helper()
	doc, ok := docs[name]
	require.Truef(t, ok, "missing Flux Kustomization %q", name)
	return doc
}

func octr656NestedString(value map[string]any, path ...string) string {
	current := value
	for _, key := range path[:len(path)-1] {
		next, _ := current[key].(map[string]any)
		current = next
	}
	result, _ := current[path[len(path)-1]].(string)
	return result
}

func octr656DependencyNames(doc map[string]any) []string {
	spec, _ := doc["spec"].(map[string]any)
	dependencies, _ := spec["dependsOn"].([]any)
	names := make([]string, 0, len(dependencies))
	for _, dependency := range dependencies {
		item, _ := dependency.(map[string]any)
		name, _ := item["name"].(string)
		names = append(names, name)
	}
	return names
}
