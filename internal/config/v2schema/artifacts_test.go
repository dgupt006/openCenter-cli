package v2schema

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/defaults"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

func TestCheckedInSchemaIsCurrent(t *testing.T) {
	root := repoRoot(t)
	if err := CheckFile(filepath.Join(root, "schema", "opencenter-v2.schema.json"), Options{}); err != nil {
		t.Fatalf("checked-in v2 schema is not current: %v", err)
	}
}

func TestV2ExampleFixturesLoad(t *testing.T) {
	root := repoRoot(t)
	paths, err := filepath.Glob(filepath.Join(root, "testdata", "config", "v2", "*.yaml"))
	if err != nil {
		t.Fatalf("glob v2 examples: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("expected at least one v2 example fixture in testdata/config/v2")
	}

	loader := v2.NewConfigLoader(defaults.NewRegistry())
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			if _, err := loader.LoadFromFile(path); err != nil {
				t.Fatalf("LoadFromFile(%s) error = %v", path, err)
			}
		})
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not locate repo root")
		}
		dir = parent
	}
}

func TestCheckedInSchemaContainsStorageContracts(t *testing.T) {
	root := repoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "schema", "opencenter-v2.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var schema map[string]any
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	services := schemaObject(schema, "properties", "opencenter", "properties", "services", "properties")
	for _, service := range []string{"loki", "tempo", "etcd-backup", "velero"} {
		fields := schemaObject(services, service, "properties")
		for _, field := range []string{"s3_endpoint", "s3_credential_id"} {
			if _, ok := fields[field]; !ok {
				t.Fatalf("schema missing opencenter.services.%s.%s", service, field)
			}
		}
	}
	for _, field := range []string{"etcd_backup", "velero"} {
		fields := schemaObject(schema, "properties", "secrets", "properties", field, "properties")
		for _, secret := range []string{"access_key_id", "secret_access_key"} {
			if _, ok := fields[secret]; !ok {
				t.Fatalf("schema missing secrets.%s.%s", field, secret)
			}
		}
	}
}

func schemaObject(root map[string]any, path ...string) map[string]any {
	value := any(root)
	for _, key := range path {
		object, ok := value.(map[string]any)
		if !ok {
			panic("schema path is not an object: " + key)
		}
		value = object[key]
	}
	object, ok := value.(map[string]any)
	if !ok {
		panic("schema path does not resolve to an object")
	}
	return object
}
