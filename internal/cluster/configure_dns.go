package cluster

import (
	"context"
	"sort"
	"strings"

	"github.com/opencenter-cloud/opencenter-cli/internal/cluster/orchestration"
	configservices "github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

type dnsCapabilityHandler struct {
	registry *configservices.ServiceProviderRegistry
}

func newDNSCapabilityHandler(registry *configservices.ServiceProviderRegistry) orchestration.CapabilityHandler {
	return &dnsCapabilityHandler{registry: registry}
}

func (h *dnsCapabilityHandler) Name() string {
	return "dns"
}

func (h *dnsCapabilityHandler) Applies(cfg *v2.Config, providerCtx orchestration.ProviderContext) bool {
	return certManagerConfig(cfg) != nil && certManagerConfig(cfg).Enabled
}

func (h *dnsCapabilityHandler) Discover(ctx context.Context, cfg *v2.Config, providerCtx orchestration.ProviderContext) (orchestration.DiscoveryResult, error) {
	return orchestration.DiscoveryResult{}, nil
}

func (h *dnsCapabilityHandler) Prompts(cfg *v2.Config, providerCtx orchestration.ProviderContext, discovery orchestration.DiscoveryResult) []orchestration.PromptSpec {
	certManager := certManagerConfig(cfg)
	if certManager == nil {
		return nil
	}

	infraProvider := configservices.InfrastructureProvider(strings.ToLower(strings.TrimSpace(providerCtx.Provider)))
	defaultProvider := strings.TrimSpace(certManager.DNSProvider)
	if defaultProvider == "" {
		defaultProvider = h.defaultDNSProvider(cfg, providerCtx)
	}

	if strings.TrimSpace(certManager.DNSProvider) == "" {
		options := make([]orchestration.PromptOption, 0)
		for _, provider := range h.registry.GetCompatibleProviders("cert-manager", "dns", infraProvider) {
			options = append(options, orchestration.PromptOption{Value: string(provider), Label: strings.ToUpper(string(provider))})
		}
		return []orchestration.PromptSpec{
			{
				ID:       "dns.provider",
				Group:    configureGroupDNS,
				Kind:     orchestration.PromptKindSelect,
				Label:    "cert-manager DNS provider",
				Default:  defaultProvider,
				Required: true,
				Options:  options,
			},
		}
	}

	switch strings.TrimSpace(certManager.DNSProvider) {
	case string(configservices.DNSProviderRoute53):
		accessKey, secretKey := cfg.GetAWSApplicationCredentials()
		if strings.TrimSpace(accessKey) == "" || strings.TrimSpace(secretKey) == "" {
			return []orchestration.PromptSpec{
				{ID: "dns.route53.region", Group: configureGroupDNS, Kind: orchestration.PromptKindInput, Label: "Route53 region", Default: firstNonEmptyString(certManager.Region, cfg.OpenCenter.Meta.Region), Required: true},
				{ID: "dns.route53.access_key", Group: configureGroupDNS, Kind: orchestration.PromptKindSecret, Label: "AWS application access key", Required: true},
				{ID: "dns.route53.secret_key", Group: configureGroupDNS, Kind: orchestration.PromptKindSecret, Label: "AWS application secret key", Required: true},
			}
		}
	case string(configservices.DNSProviderCloudflare):
		if !hasCloudflareCredential(cfg) {
			return []orchestration.PromptSpec{
				{ID: "dns.cloudflare.api_token", Group: configureGroupDNS, Kind: orchestration.PromptKindSecret, Label: "Cloudflare API token", Required: true},
			}
		}
	}

	return nil
}

// hasCloudflareCredential reports whether any cert-manager Cloudflare
// credential is usable: an enabled named credential with a token, or the legacy
// flat token. ResolvedCertManagerCloudflareCredentials only surfaces the flat
// field when dns_provider is already cloudflare, so the named map is checked
// directly here.
func hasCloudflareCredential(cfg *v2.Config) bool {
	for _, cred := range cfg.Secrets.CertManager.Cloudflare {
		if cred.Enabled && strings.TrimSpace(cred.APIToken) != "" {
			return true
		}
	}
	return strings.TrimSpace(cfg.Secrets.CertManager.CloudflareAPIToken) != ""
}

func (h *dnsCapabilityHandler) ApplyAnswers(cfg *v2.Config, answers orchestration.PromptAnswers, providerCtx orchestration.ProviderContext) (orchestration.ChangeSet, error) {
	certManager := certManagerConfig(cfg)
	if certManager == nil {
		return orchestration.ChangeSet{}, nil
	}

	changes := orchestration.ChangeSet{}

	selectedProvider := strings.TrimSpace(answers["dns.provider"])
	if selectedProvider == "" {
		selectedProvider = strings.TrimSpace(certManager.DNSProvider)
	}
	if selectedProvider == "" {
		selectedProvider = h.defaultDNSProvider(cfg, providerCtx)
	}

	if selectedProvider != "" {
		changes.Patches = append(changes.Patches, orchestration.ConfigPatch{Group: configureGroupDNS, Path: "opencenter.services.cert-manager.dns_provider", Label: "DNS provider", Value: selectedProvider})
	}

	switch selectedProvider {
	case string(configservices.DNSProviderDesignate):
		dnsZone := firstNonEmptyString(cfg.OpenCenter.Infrastructure.Cloud.OpenStack.DNSZoneName, cfg.OpenCenter.Infrastructure.Networking.DNSZoneName)
		changes.Patches = append(changes.Patches,
			orchestration.ConfigPatch{Group: configureGroupDNS, Path: "opencenter.infrastructure.networking.use_designate", Label: "Use Designate", Value: "true"},
			orchestration.ConfigPatch{Group: configureGroupDNS, Path: "opencenter.infrastructure.cloud.openstack.use_designate", Label: "Use Designate", Value: "true"},
			orchestration.ConfigPatch{Group: configureGroupDNS, Path: "opencenter.infrastructure.cloud.openstack.networking.designate.dns_zone_name", Label: "DNS zone", Value: dnsZone},
			orchestration.ConfigPatch{Group: configureGroupDNS, Path: "opencenter.infrastructure.cloud.openstack.dns_zone_name", Label: "DNS zone", Value: dnsZone},
		)
	case string(configservices.DNSProviderRoute53):
		region := firstNonEmptyString(strings.TrimSpace(answers["dns.route53.region"]), certManager.Region, cfg.OpenCenter.Meta.Region)
		accessKey := firstNonEmptyString(strings.TrimSpace(answers["dns.route53.access_key"]), firstConfiguredAWSCredential(cfg))
		secretKey := firstNonEmptyString(strings.TrimSpace(answers["dns.route53.secret_key"]), secondConfiguredAWSCredential(cfg))
		changes.Patches = append(changes.Patches,
			orchestration.ConfigPatch{Group: configureGroupDNS, Path: "opencenter.services.cert-manager.region", Label: "Route53 region", Value: region},
			orchestration.ConfigPatch{Group: configureGroupDNS, Path: "secrets.global.aws.application.region", Label: "AWS application region", Value: region},
			orchestration.ConfigPatch{Group: configureGroupDNS, Path: "secrets.global.aws.application.access_key", Label: "AWS application access key", Value: accessKey, Masked: true},
			orchestration.ConfigPatch{Group: configureGroupDNS, Path: "secrets.global.aws.application.secret_access_key", Label: "AWS application secret access key", Value: secretKey, Masked: true},
			// The rendered ClusterIssuer reads the named credential, so the guided
			// flow has to populate it: dns_provider alone used to leave the
			// credential disabled and no issuer was rendered.
			orchestration.ConfigPatch{Group: configureGroupDNS, Path: "secrets.cert_manager.aws." + selectedProvider + ".enabled", Label: "Route53 credential enabled", Value: "true"},
			orchestration.ConfigPatch{Group: configureGroupDNS, Path: "secrets.cert_manager.aws." + selectedProvider + ".aws_access_key", Label: "Route53 access key", Value: accessKey, Masked: true},
			orchestration.ConfigPatch{Group: configureGroupDNS, Path: "secrets.cert_manager.aws." + selectedProvider + ".aws_secret_access_key", Label: "Route53 secret key", Value: secretKey, Masked: true},
			orchestration.ConfigPatch{Group: configureGroupDNS, Path: "secrets.cert_manager.aws." + selectedProvider + ".region", Label: "Route53 credential region", Value: region},
			orchestration.ConfigPatch{Group: configureGroupDNS, Path: "opencenter.infrastructure.networking.use_designate", Label: "Use Designate", Value: "false"},
			orchestration.ConfigPatch{Group: configureGroupDNS, Path: "opencenter.infrastructure.cloud.openstack.use_designate", Label: "Use Designate", Value: "false"},
		)
	case string(configservices.DNSProviderCloudflare):
		token := firstNonEmptyString(strings.TrimSpace(answers["dns.cloudflare.api_token"]), firstConfiguredCloudflareToken(cfg))
		changes.Patches = append(changes.Patches,
			orchestration.ConfigPatch{Group: configureGroupDNS, Path: "secrets.cert_manager.cloudflare." + selectedProvider + ".enabled", Label: "Cloudflare credential enabled", Value: "true"},
			orchestration.ConfigPatch{Group: configureGroupDNS, Path: "secrets.cert_manager.cloudflare." + selectedProvider + ".api_token", Label: "Cloudflare API token", Value: token, Masked: true},
			orchestration.ConfigPatch{Group: configureGroupDNS, Path: "opencenter.infrastructure.networking.use_designate", Label: "Use Designate", Value: "false"},
			orchestration.ConfigPatch{Group: configureGroupDNS, Path: "opencenter.infrastructure.cloud.openstack.use_designate", Label: "Use Designate", Value: "false"},
		)
	}

	return changes, nil
}

// firstConfiguredAWSCredential returns the already-configured AWS application
// access key, so re-running configure without new secrets keeps the credential
// populated instead of blanking it.
func firstConfiguredAWSCredential(cfg *v2.Config) string {
	accessKey, _ := cfg.GetAWSApplicationCredentials()
	return strings.TrimSpace(accessKey)
}

// secondConfiguredAWSCredential returns the already-configured AWS application
// secret key.
func secondConfiguredAWSCredential(cfg *v2.Config) string {
	_, secretKey := cfg.GetAWSApplicationCredentials()
	return strings.TrimSpace(secretKey)
}

// firstConfiguredCloudflareToken returns the first named Cloudflare token worth
// keeping, so re-running configure without a new token preserves it. The init
// placeholder is not worth keeping: preserving it would outrank the freshly
// entered token and leave CHANGEME in the credential.
func firstConfiguredCloudflareToken(cfg *v2.Config) string {
	names := make([]string, 0, len(cfg.Secrets.CertManager.Cloudflare))
	for name := range cfg.Secrets.CertManager.Cloudflare {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		token := strings.TrimSpace(cfg.Secrets.CertManager.Cloudflare[name].APIToken)
		if token != "" && !strings.EqualFold(token, v2.PlaceholderSecret) {
			return token
		}
	}
	flat := strings.TrimSpace(cfg.Secrets.CertManager.CloudflareAPIToken)
	if strings.EqualFold(flat, v2.PlaceholderSecret) {
		return ""
	}
	return flat
}

func (h *dnsCapabilityHandler) defaultDNSProvider(cfg *v2.Config, providerCtx orchestration.ProviderContext) string {
	if h.registry == nil {
		return ""
	}
	infraProvider := configservices.InfrastructureProvider(strings.ToLower(strings.TrimSpace(providerCtx.Provider)))
	if infraProvider == configservices.ProviderOpenStack {
		if catalog := discoveryCatalogFromMetadata(providerCtx.Discovery); catalog != nil {
			h.registry.SetDesignateAvailability(providerCtx.ClusterName, catalog.DesignateAvailable)
		}
	}

	provider, err := h.registry.AutoSelectProvider("cert-manager", "dns", infraProvider, providerCtx.ClusterName)
	if err != nil {
		return ""
	}
	return string(provider)
}

func certManagerConfig(cfg *v2.Config) *configservices.CertManagerConfig {
	if cfg == nil {
		return nil
	}
	serviceAny, ok := cfg.OpenCenter.Services["cert-manager"]
	if !ok {
		return nil
	}
	serviceCfg, _ := serviceAny.(*configservices.CertManagerConfig)
	return serviceCfg
}
