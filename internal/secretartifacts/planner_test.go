package secretartifacts

import (
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
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

func TestPlanDefaultGrafanaSecretContainsUserAndPassword(t *testing.T) {
	cfg, err := v2.NewV2Default("grafana-defaults", "kind")
	require.NoError(t, err)

	artifacts, err := Plan(cfg)
	require.NoError(t, err)
	var grafana *Artifact
	for i := range artifacts {
		if artifacts[i].LogicalService == "grafana" {
			grafana = &artifacts[i]
			break
		}
	}
	require.NotNil(t, grafana)
	require.Equal(t, "admin", grafana.Payload["admin_user"])
	require.Contains(t, grafana.Payload, "admin_password")
	require.NotEmpty(t, grafana.Payload["admin_password"])
}

func TestPlanIncludesEtcdBackupAndVeleroWorkloadSecrets(t *testing.T) {
	cfg := &v2.Config{
		OpenCenter: v2.OpenCenterConfig{Services: map[string]any{
			"etcd-backup": &services.EtcdBackupConfig{BaseConfig: services.BaseConfig{Enabled: true}, S3Host: "s3.example", S3Region: "RegionOne"},
			"velero":      &services.VeleroConfig{BaseConfig: services.BaseConfig{Enabled: true}},
		}},
		Secrets: v2.SecretsConfig{
			EtcdBackup: v2.EtcdBackupSecrets{AccessKeyID: "etcd-access", SecretAccessKey: "etcd-secret"},
			Velero:     v2.VeleroSecrets{AccessKeyID: "velero-access", SecretAccessKey: "velero-secret"},
		},
	}
	artifacts, err := Plan(cfg)
	require.NoError(t, err)
	byService := map[string]Artifact{}
	for _, artifact := range artifacts {
		byService[artifact.TargetService] = artifact
	}
	etcd := byService["etcd-backup"]
	require.Equal(t, "etcd-access", etcd.Payload["ACCESS_KEY"])
	require.Equal(t, "s3.example", etcd.Payload["S3_HOST"])
	velero := byService["velero"]
	require.Equal(t, "[default]\naws_access_key_id=velero-access\naws_secret_access_key=velero-secret\n", velero.Payload["cloud"])
}
