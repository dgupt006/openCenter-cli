package services

import (
	"github.com/opencenter-cloud/opencenter-cli/internal/config/registry"
)

// CertManagerConfig extends BaseConfig with cert-manager configuration.
type CertManagerConfig struct {
	BaseConfig `yaml:",inline"`

	LetsEncryptServer         string                           `yaml:"letsencrypt_server,omitempty" json:"letsencrypt_server,omitempty" jsonschema:"description=LetsEncrypt ACME server URL,default=https://acme-v02.api.letsencrypt.org/directory"`
	Email                     string                           `yaml:"email,omitempty" json:"email,omitempty" jsonschema:"description=Email for LetsEncrypt registration"`
	Region                    string                           `yaml:"region,omitempty" json:"region,omitempty" jsonschema:"description=Cloud region for DNS validation (e.g. AWS Route53 region)"`
	DNSZones                  []string                         `yaml:"dns_zones,omitempty" json:"dns_zones,omitempty" jsonschema:"description=DNS zones for certificate validation"`
	CreateClusterIssuer       bool                             `yaml:"create_cluster_issuer,omitempty" json:"create_cluster_issuer,omitempty" jsonschema:"description=Create external ClusterIssuer resource,default=true"`
	Issuers                   []CertIssuer                     `yaml:"issuers,omitempty" json:"issuers,omitempty" jsonschema:"description=List of certificate issuers"`
	DNSProvider               string                           `yaml:"dns_provider,omitempty" json:"dns_provider,omitempty" jsonschema:"description=DNS provider for ACME DNS-01 challenge,enum=route53,enum=designate,enum=cloudflare,enum=clouddns,enum=azuredns"`
	DefaultClusterCertificate *DefaultClusterCertificateConfig `yaml:"default_cluster_certificate,omitempty" json:"default_cluster_certificate,omitempty" jsonschema:"description=Default cluster certificate configuration"`
}

// DefaultClusterCertificateConfig configures generation of a cluster-wide default wildcard certificate.
type DefaultClusterCertificateConfig struct {
	Enabled    bool     `yaml:"enabled" json:"enabled" jsonschema:"description=Enable generation of a default cluster-wide wildcard certificate"`
	SecretName string   `yaml:"secret_name,omitempty" json:"secret_name,omitempty" jsonschema:"description=Secret name for the default cluster certificate,default=default-cluster-tls"`
	IssuerRef  string   `yaml:"issuer_ref,omitempty" json:"issuer_ref,omitempty" jsonschema:"description=ClusterIssuer name to use; defaults to the configured Route53/ACME issuer"`
	DNSNames   []string `yaml:"dns_names,omitempty" json:"dns_names,omitempty" jsonschema:"description=Additional DNS names for the certificate"`
}

func (c *CertManagerConfig) DefaultClusterCertificateEnabled() bool {
	return c != nil && c.DefaultClusterCertificate != nil && c.DefaultClusterCertificate.Enabled
}

func (c *CertManagerConfig) DefaultClusterCertificateSecretName() string {
	if c != nil && c.DefaultClusterCertificate != nil && c.DefaultClusterCertificate.SecretName != "" {
		return c.DefaultClusterCertificate.SecretName
	}
	return "default-cluster-tls"
}

// CertIssuer represents a certificate issuer configuration.
type CertIssuer struct {
	Name   string `yaml:"name" json:"name" jsonschema:"description=Issuer name,required"`
	Type   string `yaml:"type" json:"type" jsonschema:"description=Issuer type,enum=letsencrypt,enum=selfsigned,enum=ca,required"`
	Server string `yaml:"server,omitempty" json:"server,omitempty" jsonschema:"description=ACME server URL for LetsEncrypt issuers"`
}

func init() {
	registry.RegisterServiceConfig("cert-manager", CertManagerConfig{})
}
