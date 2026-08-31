package gitops

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOCTR631OLMDescriptorComposesBaseAndReconcilesSingleOverlay(t *testing.T) {
	cfg := newDefaultServiceConfig(t)
	cfg.OpenCenter.GitOps.BaseRepo.URL = "https://example.test/openCenter-gitops-base.git"
	cfg.OpenCenter.GitOps.BaseRepo.Release = "v0.46.0"
	cfg.OpenCenter.GitOps.BaseRepo.Branch = ""
	cfg.OpenCenter.GitOps.Repository.URL = "https://example.test/customer-gitops.git"
	cfg.OpenCenter.GitOps.Repository.Branch = "customer-main"

	cfg.OpenCenter.GitOps.Repository.LocalDir = t.TempDir()
	require.NoError(t, RenderClusterApps(cfg))

	read := func(path string) string {
		return mustReadFile(t, filepath.Join(cfg.OpenCenter.GitOps.Repository.LocalDir, "applications", "overlays", cfg.ClusterName(), filepath.FromSlash(path)))
	}

	baseSource := read("services/sources/opencenter-olm.yaml")
	require.NotEmpty(t, baseSource)
	require.Contains(t, baseSource, "name: opencenter-olm")
	require.Contains(t, baseSource, "url: https://example.test/openCenter-gitops-base.git")
	require.Contains(t, baseSource, `tag: "v0.46.0"`)

	configSource := read("services/sources/opencenter-olm-config.yaml")
	require.NotEmpty(t, configSource)
	require.Contains(t, configSource, "name: opencenter-olm-config")
	require.Contains(t, configSource, "name: opencenter-olm")
	require.Contains(t, configSource, "fromPath: applications/base/services/olm")
	require.Contains(t, configSource, "toPath: applications/overlays/"+cfg.ClusterName()+"/services/base/olm/")
	require.Contains(t, configSource, "url: https://example.test/customer-gitops.git")
	require.Contains(t, configSource, `branch: "customer-main"`)
	require.Contains(t, configSource, "name: flux-system")

	overlay := read("services/olm/kustomization.yaml")
	require.NotEmpty(t, overlay)
	require.Contains(t, overlay, "- ../base/olm")
	for _, name := range []string{"olm-operator", "catalog-operator", "packageserver"} {
		require.Contains(t, overlay, "name: "+name)
	}
	require.Equal(t, 3, strings.Count(overlay, "path: /spec/egress/0"))
	require.Equal(t, 3, strings.Count(overlay, "protocol: TCP"))
	require.Equal(t, 3, strings.Count(overlay, "port: "+strconv.Itoa(cfg.OpenCenter.Cluster.Kubernetes.APIPort)))

	flux := read("services/fluxcd/olm.yaml")
	require.NotEmpty(t, flux)
	docs, err := decodeYAMLDocuments([]byte(flux))
	require.NoError(t, err)
	require.Len(t, docs, 1)
	require.Equal(t, "olm-base", nestedString(docs[0], "metadata", "name"))
	require.Equal(t, "opencenter-olm-config", nestedString(docs[0], "spec", "sourceRef", "name"))
	require.Equal(t, "applications/overlays/"+cfg.ClusterName()+"/services/olm", nestedString(docs[0], "spec", "path"))
	require.Empty(t, nestedValue(docs[0], "spec", "targetNamespace"))
	require.True(t, hasFluxDependency(t, docs[0], "sources"))
}

func TestOCTR631OLMOverlayBuildsForAPIPortsAndPreservesUpstreamRules(t *testing.T) {
	policyExpectations := map[string]struct {
		egressEntries int
		hasGRPC       bool
		selector      string
		ingressPort   any
	}{
		"olm-operator":     {egressEntries: 2, selector: "olm-operator", ingressPort: "metrics"},
		"catalog-operator": {egressEntries: 3, hasGRPC: true, selector: "catalog-operator", ingressPort: "metrics"},
		"packageserver":    {egressEntries: 3, hasGRPC: true, selector: "packageserver", ingressPort: 5443},
	}

	for _, tc := range []struct {
		name string
		port int
	}{
		{name: "443", port: 443},
		{name: "6443", port: 6443},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := newDefaultServiceConfig(t)
			cfg.OpenCenter.Cluster.Kubernetes.APIPort = tc.port
			repo := t.TempDir()
			cfg.OpenCenter.GitOps.Repository.LocalDir = repo
			require.NoError(t, RenderClusterApps(cfg))

			overlayRoot := filepath.Join(repo, "applications", "overlays", cfg.ClusterName(), "services", "olm")
			overlay := mustReadFile(t, filepath.Join(overlayRoot, "kustomization.yaml"))
			require.NotEmpty(t, overlay)
			require.Contains(t, overlay, "port: "+strconv.Itoa(tc.port))

			seedOCTR631OLMBaseFixture(t, repo, cfg.ClusterName())

			output := runKustomizeBuild(t, overlayRoot)
			docs, err := decodeYAMLDocuments([]byte(output))
			require.NoError(t, err)
			require.Len(t, docs, 3)
			for _, name := range []string{"olm-operator", "catalog-operator", "packageserver"} {
				expected := policyExpectations[name]
				doc := findNetworkPolicyDocument(t, docs, name)

				egress, ok := nestedValue(doc, "spec", "egress").([]any)
				require.True(t, ok, "%s egress has unexpected shape", name)
				require.Len(t, egress, expected.egressEntries)
				firstPorts, ok := nestedValue(egress[0].(map[string]any), "ports").([]any)
				require.True(t, ok, "%s first egress entry has unexpected shape", name)
				require.Len(t, firstPorts, 1)
				firstPort := firstPorts[0].(map[string]any)
				require.Equal(t, tc.port, firstPort["port"])
				require.Equal(t, "TCP", firstPort["protocol"])

				dnsPorts := egressPorts(egress[1:2])
				for _, port := range []string{"53/TCP", "53/UDP", "5353/TCP", "5353/UDP"} {
					require.Contains(t, dnsPorts, port, "%s must retain DNS port %s", name, port)
				}
				allEgressPorts := egressPorts(egress)
				if expected.hasGRPC {
					require.Contains(t, allEgressPorts, "50051/TCP", "%s must retain the gRPC egress port", name)
				} else {
					require.NotContains(t, allEgressPorts, "50051/TCP", "%s must not gain the catalog/packageserver gRPC port", name)
				}

				ingress, ok := nestedValue(doc, "spec", "ingress").([]any)
				require.True(t, ok, "%s ingress has unexpected shape", name)
				require.Len(t, ingress, 1)
				ingressPorts, ok := nestedValue(ingress[0].(map[string]any), "ports").([]any)
				require.True(t, ok, "%s ingress ports have unexpected shape", name)
				require.Len(t, ingressPorts, 1)
				ingressPort := ingressPorts[0].(map[string]any)
				require.Equal(t, expected.ingressPort, ingressPort["port"])
				require.Equal(t, "TCP", ingressPort["protocol"])
				require.Equal(t, expected.selector, nestedString(doc, "spec", "podSelector", "matchLabels", "app"))
				policyTypes, ok := nestedValue(doc, "spec", "policyTypes").([]any)
				require.True(t, ok, "%s policyTypes has unexpected shape", name)
				require.ElementsMatch(t, []any{"Ingress", "Egress"}, policyTypes)
			}
		})
	}
}

func seedOCTR631OLMBaseFixture(t *testing.T, repo, clusterName string) {
	t.Helper()
	baseRoot := filepath.Join(repo, "applications", "overlays", clusterName, "services", "base", "olm")
	require.NoError(t, os.MkdirAll(baseRoot, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(baseRoot, "kustomization.yaml"), []byte("apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\nresources:\n  - network-policies.yaml\n"), 0o644))

	const policies = `apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: olm-operator
  namespace: olm
spec:
  podSelector:
    matchLabels:
      app: olm-operator
  policyTypes:
    - Ingress
    - Egress
  ingress:
    - ports:
        - protocol: TCP
          port: metrics
  egress:
    - {}
    - ports:
        - protocol: TCP
          port: 53
        - protocol: UDP
          port: 53
        - protocol: TCP
          port: 5353
        - protocol: UDP
          port: 5353
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: catalog-operator
  namespace: olm
spec:
  podSelector:
    matchLabels:
      app: catalog-operator
  policyTypes:
    - Ingress
    - Egress
  ingress:
    - ports:
        - protocol: TCP
          port: metrics
  egress:
    - {}
    - ports:
        - protocol: TCP
          port: 53
        - protocol: UDP
          port: 53
        - protocol: TCP
          port: 5353
        - protocol: UDP
          port: 5353
    - ports:
        - protocol: TCP
          port: 50051
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: packageserver
  namespace: olm
spec:
  podSelector:
    matchLabels:
      app: packageserver
  policyTypes:
    - Ingress
    - Egress
  ingress:
    - ports:
        - protocol: TCP
          port: 5443
  egress:
    - {}
    - ports:
        - protocol: TCP
          port: 53
        - protocol: UDP
          port: 53
        - protocol: TCP
          port: 5353
        - protocol: UDP
          port: 5353
    - ports:
        - protocol: TCP
          port: 50051
`
	require.NoError(t, os.WriteFile(filepath.Join(baseRoot, "network-policies.yaml"), []byte(policies), 0o644))
}

func findNetworkPolicyDocument(t *testing.T, docs []map[string]any, name string) map[string]any {
	t.Helper()
	for _, doc := range docs {
		if nestedString(doc, "metadata", "name") == name {
			return doc
		}
	}
	t.Fatalf("NetworkPolicy %q not found", name)
	return nil
}

func egressPorts(entries []any) []string {
	var ports []string
	for _, entry := range entries {
		values, _ := nestedValue(entry.(map[string]any), "ports").([]any)
		for _, value := range values {
			port := value.(map[string]any)
			ports = append(ports, strconv.Itoa(int(port["port"].(int)))+"/"+port["protocol"].(string))
		}
	}
	return ports
}
