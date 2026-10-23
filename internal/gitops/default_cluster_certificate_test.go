package gitops

import (
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/stretchr/testify/require"
)

func TestDefaultClusterCertificateViaCertManager(t *testing.T) {
	cfg := v2.Config{
		OpenCenter: v2.OpenCenterConfig{
			Cluster: v2.ClusterConfig{
				ClusterName: "test-cluster",
				ClusterFQDN: "test.example.com",
			},
			Services: v2.ServiceMap{
				"gateway": &services.GatewayConfig{
					BaseConfig: services.BaseConfig{Enabled: true},
				},
				"cert-manager": &services.CertManagerConfig{
					BaseConfig: services.BaseConfig{Enabled: true},
					DefaultClusterCertificate: &services.DefaultClusterCertificateConfig{
						Enabled: true,
					},
				},
			},
		},
		Secrets: v2.SecretsConfig{
			CertManager: v2.CertManagerSecrets{
				AWS: map[string]v2.CertManagerAWSCredential{
					"route53": {
						Enabled:            true,
						AWSAccessKey:       "AKIAAWSKEY",
						AWSSecretAccessKey: "secretkey",
						Region:             "us-east-1",
						DNSZones:           []string{"test.example.com"},
					},
				},
			},
		},
	}

	files, err := gatewayOverlayFilesRenderer(cfg)
	require.NoError(t, err)

	// Verify default-cluster-certificate.yaml was rendered
	certContent, ok := files["default-cluster-certificate.yaml"]
	require.True(t, ok, "default-cluster-certificate.yaml must be rendered when default cluster certificate is enabled")

	docs, err := decodeYAMLDocuments([]byte(certContent))
	require.NoError(t, err)
	require.Len(t, docs, 1)

	cert := docs[0]
	require.Equal(t, "Certificate", cert["kind"])
	require.Equal(t, "default-cluster-certificate", nestedString(cert, "metadata", "name"))
	require.Equal(t, "rackspace-system", nestedString(cert, "metadata", "namespace"))
	require.Equal(t, "default-cluster-tls", nestedString(cert, "spec", "secretName"))
	require.Equal(t, "letsencrypt-route53", nestedString(cert, "spec", "issuerRef", "name"))
	require.Equal(t, "ClusterIssuer", nestedString(cert, "spec", "issuerRef", "kind"))

	dnsNames, ok := nestedValue(cert, "spec", "dnsNames").([]any)
	require.True(t, ok)
	require.ElementsMatch(t, []any{"test.example.com", "*.test.example.com"}, dnsNames)

	// Verify gateway kustomization includes the certificate
	kust, err := gatewayKustomizationRenderer(cfg)
	require.NoError(t, err)
	require.Contains(t, kust, "- \"default-cluster-certificate.yaml\"")

	// Verify gateway.yaml omits cluster-issuer annotation and uses default-cluster-tls
	gwContent := files["gateway.yaml"]
	require.NotEmpty(t, gwContent)
	require.NotContains(t, gwContent, "cert-manager.io/cluster-issuer")
	require.Contains(t, gwContent, "name: default-cluster-tls")
}

func TestDefaultClusterCertificateViaGateway(t *testing.T) {
	cfg := v2.Config{
		OpenCenter: v2.OpenCenterConfig{
			Cluster: v2.ClusterConfig{
				ClusterName: "test-cluster",
				ClusterFQDN: "test.example.com",
			},
			Services: v2.ServiceMap{
				"gateway": &services.GatewayConfig{
					BaseConfig: services.BaseConfig{Enabled: true},
					TLS: &services.GatewayTLSConfig{
						DefaultClusterCertificate: true,
					},
				},
				"cert-manager": &services.CertManagerConfig{
					BaseConfig:  services.BaseConfig{Enabled: true},
					DNSProvider: "route53",
				},
			},
		},
	}

	files, err := gatewayOverlayFilesRenderer(cfg)
	require.NoError(t, err)

	certContent, ok := files["default-cluster-certificate.yaml"]
	require.True(t, ok)
	require.Contains(t, certContent, "name: default-cluster-certificate")
	require.Contains(t, certContent, "secretName: default-cluster-tls")
	require.Contains(t, certContent, "name: rackspace-ca")

	gwContent := files["gateway.yaml"]
	require.NotEmpty(t, gwContent)
	require.NotContains(t, gwContent, "cert-manager.io/cluster-issuer")
	require.Contains(t, gwContent, "name: default-cluster-tls")
}

func TestRoute53IssuerResolvesAsDefaultIssuer(t *testing.T) {
	cfg := v2.Config{
		OpenCenter: v2.OpenCenterConfig{
			Cluster: v2.ClusterConfig{
				ClusterName: "ed-2",
				ClusterFQDN: "raxai.ed-2.sjc3.k8s.opencenter.cloud",
			},
			Services: v2.ServiceMap{
				"gateway": &services.GatewayConfig{
					BaseConfig: services.BaseConfig{Enabled: true},
				},
				"cert-manager": &services.CertManagerConfig{
					BaseConfig:  services.BaseConfig{Enabled: true},
					DNSProvider: "route53",
				},
			},
		},
		Secrets: v2.SecretsConfig{
			CertManager: v2.CertManagerSecrets{
				AWS: map[string]v2.CertManagerAWSCredential{
					"ed-2": {
						Enabled:            true,
						AWSAccessKey:       "AKIAAWSKEY",
						AWSSecretAccessKey: "secretkey",
						Region:             "us-east-1",
						DNSZones:           []string{"raxai.ed-2.sjc3.k8s.opencenter.cloud"},
					},
				},
			},
		},
	}

	issuer := gatewayDefaultIssuer(cfg)
	require.Equal(t, "letsencrypt-ed-2", issuer)

	files, err := gatewayOverlayFilesRenderer(cfg)
	require.NoError(t, err)
	gwContent := files["gateway.yaml"]
	require.Contains(t, gwContent, "cert-manager.io/cluster-issuer: letsencrypt-ed-2")
}

func TestGatewayDefaultIssuerDefaultsToRackspaceCAWithoutACME(t *testing.T) {
	cfg := v2.Config{
		OpenCenter: v2.OpenCenterConfig{
			Cluster: v2.ClusterConfig{
				ClusterName: "ed-2",
				ClusterFQDN: "raxai.ed-2.sjc3.k8s.opencenter.cloud",
			},
			Services: v2.ServiceMap{
				"gateway": &services.GatewayConfig{
					BaseConfig: services.BaseConfig{Enabled: true},
				},
				"cert-manager": &services.CertManagerConfig{
					BaseConfig: services.BaseConfig{Enabled: true},
				},
			},
		},
		Secrets: v2.SecretsConfig{
			CertManager: v2.CertManagerSecrets{
				AWS: map[string]v2.CertManagerAWSCredential{
					"ed-2": {
						Enabled:            true,
						AWSAccessKey:       "AKIAAWSKEY",
						AWSSecretAccessKey: "secretkey",
						Region:             "us-east-1",
						DNSZones:           []string{"raxai.ed-2.sjc3.k8s.opencenter.cloud"},
					},
				},
			},
		},
	}

	issuer := gatewayDefaultIssuer(cfg)
	require.Equal(t, "rackspace-ca", issuer)
}

func TestRenderDefaultClusterCertificateEmptyDNSNamesReturnsError(t *testing.T) {
	cfg := v2.Config{
		OpenCenter: v2.OpenCenterConfig{
			Cluster: v2.ClusterConfig{
				ClusterName: "ed-2",
				ClusterFQDN: "",
			},
			Services: v2.ServiceMap{
				"gateway": &services.GatewayConfig{
					BaseConfig: services.BaseConfig{Enabled: true},
				},
				"cert-manager": &services.CertManagerConfig{
					BaseConfig: services.BaseConfig{Enabled: true},
				},
			},
		},
	}

	_, err := renderDefaultClusterCertificate(cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "requires at least one DNS name")
}
