package v2schema

import "testing"

func TestGeneratedSchemaContainsHarborStorageContract(t *testing.T) {
	schema := generatedSchemaMap(t)
	harbor := schemaAt(t, schema, "properties", "opencenter", "properties", "services", "properties", "harbor")
	properties := schemaAt(t, harbor, "properties")

	for _, field := range []string{
		"registry_volume_size",
		"jobservice_volume_size",
		"database_volume_size",
		"redis_volume_size",
		"trivy_volume_size",
	} {
		fieldSchema := schemaAt(t, properties, field)
		if fieldSchema["type"] != "integer" {
			t.Fatalf("Harbor field %q type = %v, want integer", field, fieldSchema["type"])
		}
		if fieldSchema["minimum"] != float64(1) {
			t.Fatalf("Harbor field %q minimum = %v, want 1", field, fieldSchema["minimum"])
		}
	}
	storageType := schemaAt(t, properties, "storage_type")
	if got := stringSliceAt(t, storageType, "enum"); len(got) != 1 || got[0] != "s3" {
		t.Fatalf("Harbor storage_type enum = %v, want [s3]", got)
	}
	s3Endpoint := schemaAt(t, properties, "s3_endpoint")
	if s3Endpoint["format"] != "uri" {
		t.Fatalf("Harbor s3_endpoint format = %v, want uri", s3Endpoint["format"])
	}
	storageClass := schemaAt(t, properties, "storage_class")
	if storageClass["type"] != "string" {
		t.Fatalf("Harbor storage_class type = %v, want string", storageClass["type"])
	}
}
