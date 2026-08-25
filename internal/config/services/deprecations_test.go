package services

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDeprecatedServiceConfigKeysAreOrderedAndLookedUp(t *testing.T) {
	entries := DeprecatedServiceConfigKeys()
	if len(entries) == 0 {
		t.Fatal("deprecation registry is empty")
	}
	for i, entry := range entries {
		if entry.Key == "" || entry.Reason == "" || entry.Guidance == "" {
			t.Fatalf("registry entry %d is incomplete: %+v", i, entry)
		}
		got, ok := LookupDeprecatedServiceConfigKey(entry.Key)
		if !ok || got != entry {
			t.Fatalf("lookup(%q) = %+v, %v; want %+v, true", entry.Key, got, ok, entry)
		}
	}
	if _, ok := LookupDeprecatedServiceConfigKey("extra_dependencies"); ok {
		t.Fatal("extra_dependencies should remain supported user configuration")
	}
	if _, ok := LookupDeprecatedServiceConfigKey("conditional_dependencies"); ok {
		t.Fatal("conditional_dependencies should remain supported user configuration")
	}
}

func TestDeprecatedConfigWarningsOnlyFindExplicitUserKeys(t *testing.T) {
	data := []byte(`schema_version: "2.0"
opencenter:
  services:
    metallb:
      custom_resources: [secret.yaml]
      extra_dependencies: [network]
  managed_services:
    olm:
      kustomization_content: generated
`)
	warnings := DeprecatedConfigWarnings(data)
	if len(warnings) != 2 {
		t.Fatalf("got %d warnings, want 2: %+v", len(warnings), warnings)
	}
	if warnings[0].Path != "opencenter.services.metallb.custom_resources" {
		t.Fatalf("first warning path = %q", warnings[0].Path)
	}
	if warnings[1].Path != "opencenter.managed_services.olm.kustomization_content" {
		t.Fatalf("second warning path = %q", warnings[1].Path)
	}
}

func TestBuiltInDefaultsAloneProduceNoDeprecatedWarnings(t *testing.T) {
	data := []byte(`schema_version: "2.0"
opencenter:
  services:
    kube-prometheus-stack:
      enabled: true
    olm:
      enabled: true
`)
	if warnings := DeprecatedConfigWarnings(data); len(warnings) != 0 {
		t.Fatalf("built-in defaults must not produce warnings: %+v", warnings)
	}
}

func TestDeprecatedRegistryMatchesCheckedInSchema(t *testing.T) {
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	path := filepath.Join(filepath.Dir(sourceFile), "../../../schema/opencenter-v2.schema.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read checked-in schema: %v", err)
	}
	var schema map[string]any
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("decode checked-in schema: %v", err)
	}
	servicesSchema := schemaMapAt(t, schema, "properties", "opencenter", "properties", "services", "properties", "metallb", "properties")
	for _, entry := range DeprecatedServiceConfigKeys() {
		property := schemaMapAt(t, servicesSchema, entry.Key)
		if property["deprecated"] != true {
			t.Errorf("schema property %q is not deprecated: %v", entry.Key, property)
		}
		description, _ := property["description"].(string)
		if !strings.Contains(description, entry.Reason) || !strings.Contains(description, entry.Guidance) {
			t.Errorf("schema property %q description does not match registry: %q", entry.Key, description)
		}
	}
}

func schemaMapAt(t *testing.T, root map[string]any, path ...string) map[string]any {
	t.Helper()
	current := root
	for _, key := range path {
		next, ok := current[key].(map[string]any)
		if !ok {
			t.Fatalf("schema path %q is missing or not an object", strings.Join(path, "."))
		}
		current = next
	}
	return current
}
