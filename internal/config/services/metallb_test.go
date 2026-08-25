package services

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestMetalLBConfigRoundTrip(t *testing.T) {
	input := `enabled: true
namespace: metallb-system
ip_address_pools:
  - name: public-pool
    addresses: ["72.4.119.48/28"]
  - name: private-pool
    addresses: ["10.97.6.61/32"]
    auto_assign: false
l2_advertisements:
  - name: public-pool-l2
    ip_address_pools: [public-pool]
    interfaces: [metal.105]
`

	var config MetalLBConfig
	require.NoError(t, yaml.Unmarshal([]byte(input), &config))
	require.Len(t, config.IPAddressPools, 2)
	require.Len(t, config.L2Advertisements, 1)
	require.Nil(t, config.IPAddressPools[0].AutoAssign)
	require.True(t, config.IPAddressPools[0].GetAutoAssign())
	require.NotNil(t, config.IPAddressPools[1].AutoAssign)
	require.False(t, config.IPAddressPools[1].GetAutoAssign())

	output, err := yaml.Marshal(&config)
	require.NoError(t, err)
	var roundTripped MetalLBConfig
	require.NoError(t, yaml.Unmarshal(output, &roundTripped))
	require.Equal(t, config, roundTripped)
}
