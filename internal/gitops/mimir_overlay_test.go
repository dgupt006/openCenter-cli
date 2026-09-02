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

// TestMimirDisablesBundledKafkaWhenExternalEnabled verifies that when external
// kafka-cluster is enabled, the mimir override emits top-level kafka.enabled:
// false so the chart's bundled demo Kafka broker is not deployed.
func TestMimirDisablesBundledKafkaWhenExternalEnabled(t *testing.T) {
	cfg, err := v2.NewV2Default("k8s-mimir", "openstack")
	require.NoError(t, err)
	mimir, ok := cfg.OpenCenter.Services["mimir"].(*configservices.DefaultServiceConfig)
	require.True(t, ok)
	mimir.Enabled = true
	kafka, ok := cfg.OpenCenter.Services["kafka-cluster"].(*configservices.DefaultServiceConfig)
	require.True(t, ok)
	kafka.Enabled = true

	values := readMimirOverrideValues(t, *cfg)
	require.Contains(t, values, "kafka:\n    enabled: false",
		"mimir override must disable the bundled Kafka broker at top level when external kafka-cluster is enabled:\n%s", values)
}

// TestMimirKeepsBundledKafkaWhenExternalDisabled verifies that when external
// kafka-cluster is NOT enabled, the mimir override does not force
// kafka.enabled: false (so the chart's default Kafka remains available and the
// ingester is not stranded without any Kafka).
func TestMimirKeepsBundledKafkaWhenExternalDisabled(t *testing.T) {
	cfg, err := v2.NewV2Default("k8s-mimir", "openstack")
	require.NoError(t, err)
	mimir, ok := cfg.OpenCenter.Services["mimir"].(*configservices.DefaultServiceConfig)
	require.True(t, ok)
	mimir.Enabled = true
	// kafka-cluster stays disabled (default).

	values := readMimirOverrideValues(t, *cfg)
	require.NotContains(t, values, "kafka:\n    enabled: false",
		"mimir override must not disable bundled Kafka when external kafka-cluster is disabled:\n%s", values)
}

// TestMimirKafkaAddressUsesKafkaSystem verifies the Kafka broker address
// points at the kafka-system namespace, where kafka-cluster actually deploys
// (its kustomization/flux templates hardcode kafka-system). The
// kafka-cluster.namespace config field is not honored, so the address must not
// follow it.
func TestMimirKafkaAddressUsesKafkaSystem(t *testing.T) {
	cfg, err := v2.NewV2Default("k8s-mimir", "openstack")
	require.NoError(t, err)
	mimir, ok := cfg.OpenCenter.Services["mimir"].(*configservices.DefaultServiceConfig)
	require.True(t, ok)
	mimir.Enabled = true
	kafka, ok := cfg.OpenCenter.Services["kafka-cluster"].(*configservices.DefaultServiceConfig)
	require.True(t, ok)
	kafka.Enabled = true

	values := readMimirOverrideValues(t, *cfg)
	require.Contains(t, values, "kafka-cluster-kafka-brokers.kafka-system.svc.cluster.local:9092",
		"mimir kafka address must point at kafka-system (kafka-cluster's real namespace):\n%s", values)
}

// TestMimirKafkaAddressIgnoresConfiguredNamespace verifies that a non-default
// kafka-cluster namespace does NOT change the Kafka broker address, because
// kafka-cluster ignores the namespace field and always deploys to kafka-system.
func TestMimirKafkaAddressIgnoresConfiguredNamespace(t *testing.T) {
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
	require.Contains(t, values, "kafka-cluster-kafka-brokers.kafka-system.svc.cluster.local:9092",
		"mimir kafka address must stay at kafka-system regardless of the configured namespace:\n%s", values)
	require.NotContains(t, values, "kafka-prod",
		"mimir kafka address must not follow the (non-functional) configured namespace:\n%s", values)
}
