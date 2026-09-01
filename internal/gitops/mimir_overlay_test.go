package gitops

import (
	"path/filepath"
	"testing"

	configservices "github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/stretchr/testify/require"
)

// readMimirOverrideValues renders cluster apps with mimir enabled and returns
// the generated override-values.yaml content.
func readMimirOverrideValues(t *testing.T, cfg v2.Config) string {
	t.Helper()
	cfg.OpenCenter.GitOps.Repository.LocalDir = t.TempDir()
	require.NoError(t, RenderClusterApps(cfg))
	return mustReadFile(t, filepath.Join(
		cfg.OpenCenter.GitOps.Repository.LocalDir, "applications", "overlays",
		cfg.ClusterName(), "services", "mimir", "helm-values", "override-values.yaml"))
}

// TestMimirOverrideSetsCoreDNS verifies the mimir override sets
// global.dnsService: coredns (OCTR-707, same fix as OCTR-674 for Loki).
func TestMimirOverrideSetsCoreDNS(t *testing.T) {
	cfg, err := v2.NewV2Default("k8s-mimir", "openstack")
	require.NoError(t, err)
	mimir, ok := cfg.OpenCenter.Services["mimir"].(*configservices.DefaultServiceConfig)
	require.True(t, ok)
	mimir.Enabled = true

	values := readMimirOverrideValues(t, *cfg)
	require.Contains(t, values, "dnsService: coredns",
		"mimir override must set global.dnsService: coredns:\n%s", values)
}

// TestMimirKafkaAddressUsesConfiguredNamespace verifies the Kafka broker
// address is templated off the configured kafka-cluster namespace, not a
// hardcoded kafka-system (OCTR-707).
func TestMimirKafkaAddressUsesConfiguredNamespace(t *testing.T) {
	cfg, err := v2.NewV2Default("k8s-mimir", "openstack")
	require.NoError(t, err)
	mimir, ok := cfg.OpenCenter.Services["mimir"].(*configservices.DefaultServiceConfig)
	require.True(t, ok)
	mimir.Enabled = true
	kafka, ok := cfg.OpenCenter.Services["kafka-cluster"].(*configservices.DefaultServiceConfig)
	require.True(t, ok)
	kafka.Enabled = true

	// Default kafka-cluster namespace is "strimzi".
	values := readMimirOverrideValues(t, *cfg)
	require.Contains(t, values, "kafka-cluster-kafka-brokers.strimzi.svc.cluster.local:9092",
		"mimir kafka address must use the configured kafka-cluster namespace (strimzi):\n%s", values)
	require.NotContains(t, values, "kafka-system",
		"mimir kafka address must not hardcode kafka-system:\n%s", values)
}

// TestMimirKafkaAddressHonoursCustomNamespace verifies a non-default
// kafka-cluster namespace flows into the Kafka broker address.
func TestMimirKafkaAddressHonoursCustomNamespace(t *testing.T) {
	cfg, err := v2.NewV2Default("k8s-mimir", "openstack")
	require.NoError(t, err)
	mimir, ok := cfg.OpenCenter.Services["mimir"].(*configservices.DefaultServiceConfig)
	require.True(t, ok)
	mimir.Enabled = true
	kafka, ok := cfg.OpenCenter.Services["kafka-cluster"].(*configservices.DefaultServiceConfig)
	require.True(t, ok)
	kafka.Enabled = true
	kafka.Namespace = "kafka-prod"

	values := readMimirOverrideValues(t, *cfg)
	require.Contains(t, values, "kafka-cluster-kafka-brokers.kafka-prod.svc.cluster.local:9092",
		"mimir kafka address must honour a custom kafka-cluster namespace:\n%s", values)
}
