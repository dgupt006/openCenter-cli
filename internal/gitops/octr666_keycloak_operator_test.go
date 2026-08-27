package gitops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOCTR666KeycloakOperatorRendersForNonOrd1(t *testing.T) {
	dst := t.TempDir()
	cfg := newDefault("octr666-keycloak")
	cfg.OpenCenter.Meta.Region = "dfw"
	cfg.OpenCenter.GitOps.Repository.LocalDir = dst

	if err := RenderClusterApps(cfg); err != nil {
		t.Fatalf("RenderClusterApps() error = %v", err)
	}

	operatorDir := filepath.Join(dst, "applications", "overlays", cfg.ClusterName(), "services", "keycloak", "10-operator")
	kustomizationPath := filepath.Join(operatorDir, "kustomization.yaml")
	kustomizationDocs, err := decodeYAMLDocuments([]byte(mustReadFile(t, kustomizationPath)))
	if err != nil {
		t.Fatalf("parse %s: %v", kustomizationPath, err)
	}
	if len(kustomizationDocs) != 1 {
		t.Fatalf("expected one Kustomization document in %s, got %d", kustomizationPath, len(kustomizationDocs))
	}
	resources, ok := nestedValue(kustomizationDocs[0], "resources").([]any)
	if !ok || len(resources) != 2 || resources[0] != "./operator-group.yaml" || resources[1] != "./patch-subscription.yaml" {
		t.Fatalf("%s resources = %#v, want [./operator-group.yaml, ./patch-subscription.yaml]", kustomizationPath, resources)
	}

	// Validate the OperatorGroup is correctly scoped (OCTR-671).
	operatorGroupPath := filepath.Join(operatorDir, "operator-group.yaml")
	operatorGroupDocs, err := decodeYAMLDocuments([]byte(mustReadFile(t, operatorGroupPath)))
	if err != nil {
		t.Fatalf("parse %s: %v", operatorGroupPath, err)
	}
	if len(operatorGroupDocs) != 1 {
		t.Fatalf("expected one OperatorGroup document in %s, got %d", operatorGroupPath, len(operatorGroupDocs))
	}
	og := operatorGroupDocs[0]
	if got := og["apiVersion"]; got != "operators.coreos.com/v1" {
		t.Errorf("OperatorGroup apiVersion = %#v, want operators.coreos.com/v1", got)
	}
	if got := og["kind"]; got != "OperatorGroup" {
		t.Errorf("OperatorGroup kind = %#v, want OperatorGroup", got)
	}
	if got := nestedString(og, "metadata", "namespace"); got != "operators" {
		t.Errorf("OperatorGroup namespace = %q, want operators", got)
	}
	targetNamespaces, ok := nestedValue(og, "spec", "targetNamespaces").([]any)
	if !ok || len(targetNamespaces) != 1 || targetNamespaces[0] != "operators" {
		t.Errorf("OperatorGroup targetNamespaces = %#v, want [operators]", targetNamespaces)
	}

	subscriptionPath := filepath.Join(operatorDir, "patch-subscription.yaml")
	subscriptionContent := mustReadFile(t, subscriptionPath)
	subscriptionDocs, err := decodeYAMLDocuments([]byte(subscriptionContent))
	if err != nil {
		t.Fatalf("parse %s: %v", subscriptionPath, err)
	}
	if len(subscriptionDocs) != 1 {
		t.Fatalf("expected one Subscription document in %s, got %d", subscriptionPath, len(subscriptionDocs))
	}
	subscription := subscriptionDocs[0]
	if got := subscription["apiVersion"]; got != "operators.coreos.com/v1alpha1" {
		t.Errorf("Subscription apiVersion = %#v, want operators.coreos.com/v1alpha1", got)
	}
	if got := subscription["kind"]; got != "Subscription" {
		t.Errorf("operator resource kind = %#v, want Subscription", got)
	}
	if got := nestedString(subscription, "metadata", "name"); got != "keycloak-operator" {
		t.Errorf("Subscription metadata.name = %q, want keycloak-operator", got)
	}
	if got := nestedString(subscription, "metadata", "namespace"); got != "operators" {
		t.Errorf("Subscription metadata.namespace = %q, want operators", got)
	}
	for field, want := range map[string]string{
		"name":                "keycloak-operator",
		"channel":             "fast",
		"source":              "operatorhubio-catalog",
		"sourceNamespace":     "olm",
		"startingCSV":         "keycloak-operator.v26.4.2",
		"installPlanApproval": "Automatic",
	} {
		if got := nestedString(subscription, "spec", field); got != want {
			t.Errorf("Subscription spec.%s = %q, want %q", field, got, want)
		}
	}
	assertOrderedSubstrings(t, subscriptionContent,
		"name: keycloak-operator",
		"channel: fast",
		"source: operatorhubio-catalog",
		"sourceNamespace: olm",
		"startingCSV: keycloak-operator.v26.4.2",
		"installPlanApproval: Automatic",
	)

	entries, err := os.ReadDir(operatorDir)
	if err != nil {
		t.Fatalf("read %s: %v", operatorDir, err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		docs, err := decodeYAMLDocuments([]byte(mustReadFile(t, filepath.Join(operatorDir, entry.Name()))))
		if err != nil {
			t.Fatalf("parse generated operator resource %s: %v", entry.Name(), err)
		}
		for _, doc := range docs {
			if doc["kind"] == "CatalogSource" {
				t.Fatalf("generated Keycloak operator resources must not include a CatalogSource: %s", entry.Name())
			}
		}
	}

	fluxPath := filepath.Join(dst, "applications", "overlays", cfg.ClusterName(), "services", "fluxcd", "keycloak.yaml")
	fluxDocs, err := decodeYAMLDocuments([]byte(mustReadFile(t, fluxPath)))
	if err != nil {
		t.Fatalf("parse %s: %v", fluxPath, err)
	}

	// Validate keycloak-namespace stage exists and keycloak-postgres depends on it (OCTR-673).
	namespaceStage := findFluxKustomization(t, fluxDocs, "keycloak-namespace")
	if namespaceStage == nil {
		t.Fatalf("keycloak-namespace Kustomization not found in %s", fluxPath)
	}
	postgresStage := findFluxKustomization(t, fluxDocs, "keycloak-postgres")
	if !hasFluxDependency(t, postgresStage, "keycloak-namespace") {
		t.Fatalf("keycloak-postgres must depend on keycloak-namespace")
	}

	// Verify the namespace resource file exists.
	nsPath := filepath.Join(dst, "applications", "overlays", cfg.ClusterName(), "services", "keycloak", "namespace", "namespace.yaml")
	nsContent := mustReadFile(t, nsPath)
	if !strings.Contains(nsContent, "name: keycloak") {
		t.Errorf("namespace.yaml does not define namespace 'keycloak': %s", nsContent)
	}

	operatorStage := findFluxKustomization(t, fluxDocs, "keycloak-operator")
	assertFluxDependenciesInOrder(t, operatorStage, "keycloak-operator", "sources", "olm-base", "keycloak-postgres")
	if got := nestedString(operatorStage, "spec", "targetNamespace"); got != "operators" {
		t.Errorf("keycloak-operator targetNamespace = %q, want operators", got)
	}
	healthChecks, ok := nestedValue(operatorStage, "spec", "healthChecks").([]any)
	if !ok || len(healthChecks) != 1 {
		t.Fatalf("keycloak-operator healthChecks = %#v, want one Deployment health check", healthChecks)
	}
	healthCheck, ok := healthChecks[0].(map[string]any)
	if !ok {
		t.Fatalf("keycloak-operator health check has unexpected shape: %#v", healthChecks[0])
	}
	for field, want := range map[string]string{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"name":       "keycloak-operator",
		"namespace":  "operators",
	} {
		if got := healthCheck[field]; got != want {
			t.Errorf("keycloak-operator health check %s = %#v, want %q", field, got, want)
		}
	}

	keycloakStage := findFluxKustomization(t, fluxDocs, "keycloak-cr")
	if !hasFluxDependency(t, keycloakStage, "keycloak-operator") {
		t.Fatalf("keycloak-cr must depend on keycloak-operator")
	}
}

func assertOrderedSubstrings(t *testing.T, content string, want ...string) {
	t.Helper()
	previous := -1
	for _, substring := range want {
		index := strings.Index(content, substring)
		if index < 0 {
			t.Errorf("rendered content is missing %q", substring)
			continue
		}
		if index <= previous {
			t.Errorf("rendered content places %q out of order", substring)
		}
		previous = index
	}
}

func assertFluxDependenciesInOrder(t *testing.T, doc map[string]any, stage string, wanted ...string) {
	t.Helper()
	dependencies, ok := nestedValue(doc, "spec", "dependsOn").([]any)
	if !ok {
		t.Fatalf("%s has no spec.dependsOn list: %#v", stage, doc)
	}
	if len(dependencies) != len(wanted) {
		t.Fatalf("%s dependencies = %#v, want names in order %v", stage, dependencies, wanted)
	}
	for index, raw := range dependencies {
		dependency, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("%s has malformed dependency: %#v", stage, raw)
		}
		if got := dependency["name"]; got != wanted[index] {
			t.Errorf("%s dependency %d = %#v, want %q", stage, index, got, wanted[index])
		}
	}
}
