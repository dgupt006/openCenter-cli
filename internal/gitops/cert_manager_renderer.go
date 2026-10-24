// Copyright 2025 Victor Palma <victor.palma@rackspace.com>
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gitops

import (
	"fmt"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/secretartifacts"
)

const (
	structKind = reflect.Struct
	stringKind = reflect.String
)

// renderInlineTemplateContent renders a template without writing to disk.
func renderInlineTemplateContent(tmplStr, filename string, data any) (string, error) {
	funcMap := sprig.TxtFuncMap()
	t, err := template.New(filename).Funcs(funcMap).Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("parsing template for %s: %w", filename, err)
	}
	var buf strings.Builder
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("executing template for %s: %w", filename, err)
	}
	if strings.TrimSpace(buf.String()) == "" {
		return "", nil
	}
	return buf.String(), nil
}

// planCertManagerDynamicActions returns the dynamic cert-manager files as
// ordinary render actions. Keeping them in the plan preserves atomic promotion
// and makes single-service rendering use the same ownership model as full render.
func planCertManagerDynamicActions(cfg v2.Config, artifacts []secretartifacts.Artifact) ([]clusterAppAction, error) {
	certManagerSvc, exists := cfg.OpenCenter.Services["cert-manager"]
	if !exists || IsServiceDisabled(certManagerSvc) {
		return nil, nil
	}
	if err := validateCertManagerCredentials(cfg); err != nil {
		return nil, err
	}

	awsCreds := cfg.EnabledCertManagerAWSCredentials()
	cfCreds := cfg.EnabledCertManagerCloudflareCredentials()
	var credentials []certManagerCredentialData
	for name, cred := range awsCreds {
		credentials = append(credentials, certManagerCredentialData{
			Name: name, Provider: "aws", AWSAccessKey: cred.AWSAccessKey,
			AWSSecretAccessKey: cred.AWSSecretAccessKey, Region: cred.Region, DNSZones: cred.DNSZones,
		})
	}
	for name, cred := range cfCreds {
		credentials = append(credentials, certManagerCredentialData{
			Name: name, Provider: "cloudflare", APIToken: cred.APIToken, DNSZones: cred.DNSZones,
		})
	}
	sort.Slice(credentials, func(i, j int) bool {
		if credentials[i].Provider != credentials[j].Provider {
			return credentials[i].Provider < credentials[j].Provider
		}
		return credentials[i].Name < credentials[j].Name
	})

	certManagerConfig := extractCertManagerConfig(cfg)
	for i := range credentials {
		credentials[i].ClusterName = cfg.ClusterName()
		credentials[i].ClusterFQDN = cfg.OpenCenter.Cluster.ClusterFQDN
		credentials[i].LetsEncryptServer = certManagerConfig.letsEncryptServer
		credentials[i].Email = certManagerConfig.email
		if credentials[i].Region == "" {
			credentials[i].Region = certManagerConfig.region
		}
		credentials[i].SecretName = fmt.Sprintf("opencenter-aws-credentials-secret-%s", credentials[i].Name)
		credentials[i].AccessKeyKey = "aws-access-key"
		credentials[i].SecretAccessKeyKey = "aws-secret-access-key"
	}

	var actions []clusterAppAction
	for _, cred := range credentials {
		var templateText, filename string
		switch cred.Provider {
		case "aws":
			templateText = awsSecretTemplate
			filename = fmt.Sprintf("opencenter-aws-credentials-secret-%s.yaml", cred.Name)
		case "cloudflare":
			templateText = cloudflareSecretTemplate
			filename = fmt.Sprintf("opencenter-cloudflare-credentials-secret-%s.yaml", cred.Name)
		default:
			return nil, fmt.Errorf("unsupported provider %q", cred.Provider)
		}
		content, err := renderInlineTemplateContent(templateText, filename, cred)
		if err != nil {
			return nil, err
		}
		actions = append(actions, clusterAppAction{Owner: "service-cert-manager", Output: filepath.ToSlash(filepath.Join("services/cert-manager", filename)), Content: content})

		switch cred.Provider {
		case "aws":
			templateText = issuerAWSTemplate
		case "cloudflare":
			templateText = issuerCloudflareTemplate
		}
		filename = fmt.Sprintf("letsencrypt-%s-issuer.yaml", cred.Name)
		content, err = renderInlineTemplateContent(templateText, filename, cred)
		if err != nil {
			return nil, err
		}
		actions = append(actions, clusterAppAction{Owner: "service-cert-manager", Output: filepath.ToSlash(filepath.Join("services/cert-manager", filename)), Content: content})
	}

	if dnsProvider, ok := getStringField(certManagerSvc, "DNSProvider"); ok && dnsProvider == "designate" {
		issuerData := certManagerCredentialData{
			Name: "designate", Provider: "designate", ClusterName: cfg.ClusterName(),
			ClusterFQDN: cfg.OpenCenter.Cluster.ClusterFQDN, LetsEncryptServer: certManagerConfig.letsEncryptServer, Email: certManagerConfig.email,
		}
		content, err := renderInlineTemplateContent(issuerDesignateTemplate, "letsencrypt-designate-issuer.yaml", issuerData)
		if err != nil {
			return nil, err
		}
		actions = append(actions, clusterAppAction{Owner: "service-cert-manager", Output: "services/cert-manager/letsencrypt-designate-issuer.yaml", Content: content})
	}

	hasDesignate := false
	if dnsProvider, ok := getStringField(certManagerSvc, "DNSProvider"); ok {
		hasDesignate = dnsProvider == "designate"
	}
	hasCertManagerArtifact := false
	for _, a := range artifacts {
		if a.TargetService == "cert-manager" {
			hasCertManagerArtifact = true
			break
		}
	}
	content, err := renderInlineTemplateContent(kustomizationTemplate, "kustomization.yaml", kustomizationData{
		Credentials:           credentials,
		HasDesignate:          hasDesignate,
		HasMaterializedSecret: hasCertManagerArtifact,
	})
	if err != nil {
		return nil, err
	}
	actions = append(actions, clusterAppAction{Owner: "service-cert-manager", Output: "services/cert-manager/kustomization.yaml", Content: content})
	return actions, nil
}

// cert-manager credential secret and its corresponding issuer.
type certManagerCredentialData struct {
	// Name is the credential identifier (map key from config).
	Name string

	// Provider is "aws" or "cloudflare".
	Provider string

	// AWS-specific fields
	AWSAccessKey       string
	AWSSecretAccessKey string
	Region             string

	// Cloudflare-specific fields
	APIToken string

	// DNSZones for the issuer selector.
	DNSZones []string

	// Cluster-level fields needed by the issuer template.
	ClusterName       string
	ClusterFQDN       string
	LetsEncryptServer string
	Email             string

	SecretName         string
	AccessKeyKey       string
	SecretAccessKeyKey string
}

// certManagerServiceConfig holds extracted cert-manager service configuration.
type certManagerServiceConfig struct {
	letsEncryptServer string
	email             string
	region            string
}

// validateCertManagerCredentialName enforces the DNS-1123 label contract used by
// credential names in Kubernetes resource names and generated filenames.
func validateCertManagerCredentialName(provider, name string) error {
	if name == "" {
		return fmt.Errorf("%s credential name must not be empty", provider)
	}
	if len(name) > 63 {
		return fmt.Errorf("%s credential name %q exceeds the 63-character DNS-1123 label limit", provider, name)
	}
	isLowerAlphaNumeric := func(char byte) bool {
		return (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9')
	}
	if !isLowerAlphaNumeric(name[0]) || !isLowerAlphaNumeric(name[len(name)-1]) {
		return fmt.Errorf("%s credential name %q must start and end with a lowercase letter or digit", provider, name)
	}
	for i := 0; i < len(name); i++ {
		char := name[i]
		if !isLowerAlphaNumeric(char) && char != '-' {
			return fmt.Errorf("%s credential name %q must be a DNS-1123 label containing only lowercase letters, digits, and hyphens", provider, name)
		}
	}
	return nil
}

// validateCertManagerCredentials checks that all enabled credentials have their
// required secret fields populated and that every credential map key is safe to
// interpolate into Kubernetes resource names and generated paths.
func validateCertManagerCredentials(cfg v2.Config) error {
	var errors []string

	for name, cred := range cfg.Secrets.CertManager.AWS {
		if err := validateCertManagerCredentialName("AWS", name); err != nil {
			errors = append(errors, fmt.Sprintf("secrets.cert_manager.aws: %v", err))
		}
		if !cred.Enabled {
			continue
		}
		if cred.AWSAccessKey == "" {
			errors = append(errors, fmt.Sprintf("secrets.cert_manager.aws.%s.aws_access_key is required when enabled", name))
		}
		if cred.AWSSecretAccessKey == "" {
			errors = append(errors, fmt.Sprintf("secrets.cert_manager.aws.%s.aws_secret_access_key is required when enabled", name))
		}
	}

	for name, cred := range cfg.Secrets.CertManager.Cloudflare {
		if err := validateCertManagerCredentialName("Cloudflare", name); err != nil {
			errors = append(errors, fmt.Sprintf("secrets.cert_manager.cloudflare: %v", err))
		}
		if !cred.Enabled {
			continue
		}
		if cred.APIToken == "" {
			errors = append(errors, fmt.Sprintf("secrets.cert_manager.cloudflare.%s.api_token is required when enabled", name))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("cert-manager credential validation failed:\n  - %s", strings.Join(errors, "\n  - "))
	}
	return nil
}

// extractCertManagerConfig extracts cert-manager service config fields using reflection
// since the service config is stored as any in the ServiceMap.
func extractCertManagerConfig(cfg v2.Config) certManagerServiceConfig {
	result := certManagerServiceConfig{
		letsEncryptServer: "https://acme-v02.api.letsencrypt.org/directory",
		email:             "mpk-support@rackspace.com",
	}

	svc, exists := cfg.OpenCenter.Services["cert-manager"]
	if !exists {
		return result
	}

	if v, ok := getStringField(svc, "LetsEncryptServer"); ok && v != "" {
		result.letsEncryptServer = v
	}
	if v, ok := getStringField(svc, "Email"); ok && v != "" {
		result.email = v
	}
	if v, ok := getStringField(svc, "Region"); ok && v != "" {
		result.region = v
	}

	return result
}

// getStringField extracts a string field from a struct by name using reflection.
func getStringField(obj any, fieldName string) (string, bool) {
	if obj == nil {
		return "", false
	}
	val := derefValue(obj)
	if !val.IsValid() || val.Kind() != structKind {
		return "", false
	}
	field := val.FieldByName(fieldName)
	if !field.IsValid() || field.Kind() != stringKind {
		return "", false
	}
	return field.String(), true
}

// derefValue dereferences pointers and interfaces to get the underlying value.
func derefValue(obj any) reflect.Value {
	val := reflect.ValueOf(obj)
	for val.Kind() == reflect.Ptr || val.Kind() == reflect.Interface {
		if val.IsNil() {
			return reflect.Value{}
		}
		val = val.Elem()
	}
	return val
}

// awsSecretTemplate renders the per-credential Route53 Secret referenced by
// issuerAWSTemplate. The key names must stay in lockstep with
// certManagerCredentialData.AccessKeyKey/SecretAccessKeyKey: the ClusterIssuer
// resolves accessKeyIDSecretRef.key against this Secret, so an extra or renamed
// key here is silently ignored by cert-manager. The pre-2026.10 pair
// (access-key-id / secret-access-key) was dropped once the issuer standardized
// on the aws-* names; nothing references them any more.
const awsSecretTemplate = `apiVersion: v1
kind: Secret
metadata:
  name: opencenter-aws-credentials-secret-{{ .Name }}
type: Opaque
stringData:
  aws-access-key: {{ .AWSAccessKey | quote }}
  aws-secret-access-key: {{ .AWSSecretAccessKey | quote }}
`

const cloudflareSecretTemplate = `apiVersion: v1
kind: Secret
metadata:
  name: opencenter-cloudflare-credentials-secret-{{ .Name }}
type: Opaque
stringData:
  api-token: {{ .APIToken }}
`

const issuerAWSTemplate = `apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-{{ .Name }}
spec:
  acme:
    server: {{ .LetsEncryptServer }}
    email: {{ .Email }}
    privateKeySecretRef:
      name: letsencrypt-dns01-{{ .Name }}
    solvers:
      - dns01:
          route53:
            region: {{ .Region }}
            accessKeyIDSecretRef:
              name: "{{ .SecretName }}"
              key: {{ .AccessKeyKey }}
            secretAccessKeySecretRef:
              name: "{{ .SecretName }}"
              key: {{ .SecretAccessKeyKey }}
{{- if .DNSZones }}
        selector:
          dnsZones:
{{- range .DNSZones }}
            - {{ . }}
{{- end }}
{{- end }}
`

const issuerCloudflareTemplate = `apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-{{ .Name }}
spec:
  acme:
    server: {{ .LetsEncryptServer }}
    email: {{ .Email }}
    privateKeySecretRef:
      name: letsencrypt-dns01-{{ .Name }}
    solvers:
      - dns01:
          cloudflare:
            apiTokenSecretRef:
              name: "opencenter-cloudflare-credentials-secret-{{ .Name }}"
              key: api-token
{{- if .DNSZones }}
        selector:
          dnsZones:
{{- range .DNSZones }}
            - {{ . }}
{{- end }}
{{- end }}
`

const issuerDesignateTemplate = `apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-{{ .Name }}
spec:
  acme:
    server: {{ .LetsEncryptServer }}
    email: {{ .Email }}
    privateKeySecretRef:
      name: letsencrypt-dns01-{{ .Name }}
    solvers:
      - dns01:
          webhook:
            groupName: acme.syseleven.de
            solverName: designatedns
{{- if .DNSZones }}
        selector:
          dnsZones:
{{- range .DNSZones }}
            - {{ . }}
{{- end }}
{{- end }}
`

const kustomizationTemplate = `---
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: cert-manager
resources:
  - "./rackspace-selfsigned-issuer.yaml"
  - "./rackspace-selfsigned-ca.yaml"
  - "./rackspace-ca-issuer.yaml"
{{- if .HasDesignate }}
  - "./opencenter-openstack-designate-credentials-secret.yaml"
  - "./letsencrypt-designate-issuer.yaml"
{{- end }}
{{- range .Credentials }}
{{- if eq .Provider "aws" }}
  - "./opencenter-aws-credentials-secret-{{ .Name }}.yaml"
{{- end }}
{{- if eq .Provider "cloudflare" }}
  - "./opencenter-cloudflare-credentials-secret-{{ .Name }}.yaml"
{{- end }}
  - "./letsencrypt-{{ .Name }}-issuer.yaml"
{{- end }}
{{- if .HasMaterializedSecret }}
  # secret.yaml is the generic per-service Secret written by the secrets
  # manager (name: opencenter-cert-manager-secret) from the flat legacy
  # Secrets.CertManager fields. It intentionally coexists with the
  # per-credential Secrets rendered above: the flat fields are retained for
  # migration compatibility (see secretartifacts.certManagerPayload), while the
  # ClusterIssuers only ever reference opencenter-aws-credentials-secret-<name>.
  - "./secret.yaml"
{{- end }}
secretGenerator:
  - name: cert-manager-values-override
    files: [override.yaml=helm-values/override-values.yaml]
    options:
      disableNameSuffixHash: true
`

// kustomizationData holds the template context for the cert-manager kustomization.yaml.
type kustomizationData struct {
	Credentials           []certManagerCredentialData
	HasDesignate          bool
	HasMaterializedSecret bool
}
