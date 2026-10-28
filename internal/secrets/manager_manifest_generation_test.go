package secrets

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestGenerateSecretManifestUsesStringDataAndReadableIdentity(t *testing.T) {
	manager, _, cleanup := setupTestManager(t)
	defer cleanup()

	manifest := manager.generateSecretManifest("grafana", map[string]interface{}{
		"admin_user":     "admin",
		"admin_password": "password",
	}, map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Secret",
		"metadata": map[string]interface{}{
			"name": "grafana-admin-password",
		},
		"data": map[string]interface{}{"stale": "must-be-dropped"},
	}, serviceScopedSecretPath("grafana"))

	require.Equal(t, "v1", manifest["apiVersion"])
	require.Equal(t, "Secret", manifest["kind"])
	metadata, ok := manifest["metadata"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "grafana-admin-password", metadata["name"])
	require.NotContains(t, manifest, "data")
	stringData, ok := manifest["stringData"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "admin", stringData["admin-user"])
	require.Equal(t, "password", stringData["admin-password"])

	yamlData, err := yaml.Marshal(manifest)
	require.NoError(t, err)
	text := string(yamlData)
	require.Contains(t, text, "apiVersion: v1")
	require.Contains(t, text, "kind: Secret")
	require.Contains(t, text, "name: grafana-admin-password")
	require.Contains(t, text, "stringData:")
	require.NotContains(t, text, "\ndata:")
}

func TestGenerateSecretManifestAddsFinalValuesSecretMetadata(t *testing.T) {
	manager, _, cleanup := setupTestManager(t)
	defer cleanup()

	for _, test := range []struct {
		service   string
		name      string
		namespace string
	}{
		{service: "harbor", name: "opencenter-harbor-secret", namespace: "harbor"},
		{service: "loki", name: "opencenter-loki-secret", namespace: "observability"},
	} {
		t.Run(test.service, func(t *testing.T) {
			manifest := manager.generateSecretManifest(test.service, map[string]interface{}{
				"values.yaml": "loki:\n  storage:\n    s3:\n      accessKeyId: access\n",
			}, nil, serviceScopedSecretPath(test.service))

			metadata, ok := manifest["metadata"].(map[string]interface{})
			require.True(t, ok)
			require.Equal(t, test.name, metadata["name"])
			require.Equal(t, test.namespace, metadata["namespace"])
			labels, ok := metadata["labels"].(map[string]interface{})
			require.True(t, ok)
			require.Equal(t, "Enabled", labels["reconcile.fluxcd.io/watch"])
			data, ok := manifest["stringData"].(map[string]interface{})
			require.True(t, ok)
			require.Len(t, data, 1)
			require.Contains(t, data, "values.yaml")
		})
	}
}
