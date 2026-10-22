package v2

import (
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/defaults"
	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	"github.com/stretchr/testify/require"
)

func ownershipCfg(ccmEnabled, metallbEnabled bool) *Config {
	return &Config{OpenCenter: OpenCenterConfig{
		Infrastructure: InfrastructureConfig{Provider: "openstack"},
		Services: ServiceMap{
			"openstack-ccm": &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{Enabled: ccmEnabled}},
			"metallb":       &services.MetalLBConfig{BaseConfig: services.BaseConfig{Enabled: metallbEnabled}},
		},
	}}
}

// The ownership conflict is a deployment-time rule: Validate (and therefore
// ValidateForDeployment/ValidateGeneration) rejects the pair...
func TestValidateRejectsOpenStackCCMAndMetalLB(t *testing.T) {
	err := validateLoadBalancerOwnership(ownershipCfg(true, true))
	require.Error(t, err)
	require.Contains(t, err.Error(), "both controllers manage classless LoadBalancer Services")
}

func TestValidateAllowsOpenStackCCMWithoutMetalLB(t *testing.T) {
	require.NoError(t, validateLoadBalancerOwnership(ownershipCfg(true, false)))
}

func TestValidateAllowsMetalLBWithoutOpenStackCCM(t *testing.T) {
	require.NoError(t, validateLoadBalancerOwnership(ownershipCfg(false, true)))
}

// ...while load-time validation and renderers must accept it, so stored configs
// that enable metallb alongside the default-enabled openstack-ccm remain
// loadable and `generate --skip-validation` can still render the manifests.
func TestValidateServicesAllowsCCMAndMetalLBPairForRenderers(t *testing.T) {
	require.NoError(t, NewValidator().ValidateServices(ownershipCfg(true, true)))
}

func TestLoaderAcceptsCCMAndMetalLBPair(t *testing.T) {
	cfg, err := NewConfigLoader(defaults.NewRegistry()).LoadFromBytes(marshaledOwnershipConfig(t))
	require.NoError(t, err)
	require.True(t, isServiceEnabled(cfg, "metallb"))
	require.True(t, isServiceEnabled(cfg, "openstack-ccm"))
}

func marshaledOwnershipConfig(t *testing.T) []byte {
	cfg, err := NewV2Default("ownership-test", "openstack")
	require.NoError(t, err)
	cfg.OpenCenter.Services["metallb"] = &services.MetalLBConfig{
		BaseConfig: services.BaseConfig{Enabled: true, Namespace: "metallb-system"},
		IPAddressPools: []services.IPAddressPool{
			{Name: "public-pool", Addresses: []string{"72.4.119.48/28"}},
		},
	}
	data, err := MarshalPublicConfig(cfg)
	require.NoError(t, err)
	return data
}
