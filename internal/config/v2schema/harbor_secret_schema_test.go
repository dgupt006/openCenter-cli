package v2schema

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCheckedInSchemaContainsHarborSecretContract(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, "schema", "opencenter-v2.schema.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read checked-in schema: %v", err)
	}
	var schema map[string]any
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("parse checked-in schema: %v", err)
	}

	harbor := schemaAt(t, schema, "properties", "secrets", "properties", "harbor")
	properties := schemaAt(t, harbor, "properties")
	for _, field := range []string{"admin_password", "registry_password", "database_password"} {
		if _, ok := properties[field]; !ok {
			t.Fatalf("Harbor schema missing field %q: %#v", field, properties)
		}
	}
	if err := CheckFile(path, Options{}); err != nil {
		t.Fatalf("checked-in schema is stale: %v", err)
	}
}
