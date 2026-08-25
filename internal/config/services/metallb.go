package services

import (
	"github.com/opencenter-cloud/opencenter-cli/internal/config/registry"
)

// MetalLBConfig extends BaseConfig with MetalLB-specific configuration.
type MetalLBConfig struct {
	BaseConfig       `yaml:",inline"`
	IPAddressPools   []IPAddressPool   `yaml:"ip_address_pools,omitempty" json:"ip_address_pools,omitempty" jsonschema:"description=List of MetalLB IP address pools"`
	L2Advertisements []L2Advertisement `yaml:"l2_advertisements,omitempty" json:"l2_advertisements,omitempty" jsonschema:"description=List of MetalLB L2 advertisements"`
}

// IPAddressPool represents a MetalLB IP address pool.
type IPAddressPool struct {
	Name          string   `yaml:"name" json:"name" jsonschema:"description=Name of the IP address pool,required"`
	Addresses     []string `yaml:"addresses" json:"addresses" jsonschema:"description=IP ranges in CIDR or start-end form,required"`
	AutoAssign    *bool    `yaml:"auto_assign,omitempty" json:"auto_assign,omitempty" jsonschema:"description=Automatically assign IPs from this pool,default=true"`
	AvoidBuggyIPs bool     `yaml:"avoid_buggy_ips,omitempty" json:"avoid_buggy_ips,omitempty" jsonschema:"description=Avoid .0 and .255 addresses"`
}

// GetAutoAssign returns the MetalLB default of assigning addresses automatically.
func (p IPAddressPool) GetAutoAssign() bool {
	return p.AutoAssign == nil || *p.AutoAssign
}

// L2Advertisement represents a MetalLB layer-2 advertisement.
type L2Advertisement struct {
	Name           string   `yaml:"name" json:"name" jsonschema:"description=Name of the L2 advertisement,required"`
	IPAddressPools []string `yaml:"ip_address_pools,omitempty" json:"ip_address_pools,omitempty" jsonschema:"description=Pools to advertise; empty means all pools"`
	Interfaces     []string `yaml:"interfaces,omitempty" json:"interfaces,omitempty" jsonschema:"description=Node interfaces to advertise on"`
}

func init() {
	registry.RegisterServiceConfig("metallb", MetalLBConfig{})
}
