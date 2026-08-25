package secretartifacts

import (
	"testing"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/stretchr/testify/require"
)

func TestPlanRoutesAndNormalizesArtifacts(t *testing.T) {
	cfg := &v2.Config{
		Secrets: v2.SecretsConfig{
			Grafana: v2.GrafanaSecrets{AdminPassword: " grafana-password "},
			ServiceSecrets: map[string]any{
				"my_service": map[string]any{"token": "token", "empty": "  "},
			},
		},
	}

	artifacts, err := Plan(cfg)
	require.NoError(t, err)
	require.Len(t, artifacts, 2)

	var grafana, arbitrary Artifact
	for _, artifact := range artifacts {
		switch artifact.LogicalService {
		case "grafana":
			grafana = artifact
		case "my-service":
			arbitrary = artifact
		}
	}
	require.Equal(t, "kube-prometheus-stack", grafana.TargetService)
	require.Equal(t, "services/kube-prometheus-stack/secret.yaml", grafana.Path)
	require.Equal(t, " grafana-password ", grafana.Payload["admin_password"])
	require.Equal(t, "my-service", arbitrary.TargetService)
	require.Equal(t, "services/my-service/secret.yaml", arbitrary.Path)
	require.NotContains(t, arbitrary.Payload, "empty")
}

func TestPlanRejectsUnsafeServiceNames(t *testing.T) {
	_, err := Plan(&v2.Config{Secrets: v2.SecretsConfig{
		ServiceSecrets: map[string]any{"../outside": map[string]any{"token": "value"}},
	}})
	require.Error(t, err)
}
