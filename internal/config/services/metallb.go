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
	Default       bool     `yaml:"default,omitempty" json:"default,omitempty" jsonschema:"description=Mark this pool as the default pool other CLI features select when a service declares no explicit pool"`
	AutoAssign    *bool    `yaml:"auto_assign,omitempty" json:"auto_assign,omitempty" jsonschema:"description=Automatically assign IPs from this pool,default=true"`
	AvoidBuggyIPs bool     `yaml:"avoid_buggy_ips,omitempty" json:"avoid_buggy_ips,omitempty" jsonschema:"description=Avoid .0 and .255 addresses"`
}

// GetAutoAssign returns the MetalLB default of assigning addresses automatically.
func (p IPAddressPool) GetAutoAssign() bool {
	return p.AutoAssign == nil || *p.AutoAssign
}

// L2AdvertisementType is the advertisement type discriminator. Only layer-2 is
// supported today; BGP advertisements are a follow-up ticket.
const L2AdvertisementType = "l2"

// L2Advertisement represents a MetalLB layer-2 advertisement.
type L2Advertisement struct {
	Name           string         `yaml:"name" json:"name" jsonschema:"description=Name of the L2 advertisement,required"`
	Type           string         `yaml:"type,omitempty" json:"type,omitempty" jsonschema:"description=Advertisement type; only \"l2\" is supported today,default=l2"`
	IPAddressPools []string       `yaml:"ip_address_pools,omitempty" json:"ip_address_pools,omitempty" jsonschema:"description=Pools to advertise; empty means all pools"`
	Interfaces     []string       `yaml:"interfaces,omitempty" json:"interfaces,omitempty" jsonschema:"description=Node interfaces to advertise on"`
	NodeSelectors  []NodeSelector `yaml:"node_selectors,omitempty" json:"node_selectors,omitempty" jsonschema:"description=Restrict which nodes advertise the pool IPs; rendered as L2Advertisement spec.nodeSelectors. Empty means all nodes are eligible."`
}

// NodeSelector restricts which nodes advertise the load-balancer IPs for an L2
// advertisement. It mirrors a Kubernetes metav1.LabelSelector and renders as an
// entry in L2Advertisement spec.nodeSelectors.
type NodeSelector struct {
	MatchLabels      map[string]string         `yaml:"match_labels,omitempty" json:"match_labels,omitempty" jsonschema:"description=Node label key/value pairs that must all match"`
	MatchExpressions []NodeSelectorRequirement `yaml:"match_expressions,omitempty" json:"match_expressions,omitempty" jsonschema:"description=Node label selector requirements"`
}

// NodeSelectorRequirement is a single label selector requirement, mirroring a
// Kubernetes metav1.LabelSelectorRequirement.
type NodeSelectorRequirement struct {
	Key      string   `yaml:"key" json:"key" jsonschema:"description=Label key that the selector applies to,required"`
	Operator string   `yaml:"operator" json:"operator" jsonschema:"description=Selector operator; one of In, NotIn, Exists, DoesNotExist,required"`
	Values   []string `yaml:"values,omitempty" json:"values,omitempty" jsonschema:"description=Values for the operator; required for In/NotIn and must be empty for Exists/DoesNotExist"`
}

// GetType returns the advertisement type, defaulting to layer-2.
func (a L2Advertisement) GetType() string {
	if a.Type == "" {
		return L2AdvertisementType
	}
	return a.Type
}

// DefaultPoolName returns the name of the pool marked default, or the first pool
// when none is explicitly marked, or "" when no pools are defined. Other CLI
// features (per-service pool selection) use this as the fallback pool.
func (c MetalLBConfig) DefaultPoolName() string {
	if len(c.IPAddressPools) == 0 {
		return ""
	}
	for _, pool := range c.IPAddressPools {
		if pool.Default {
			return pool.Name
		}
	}
	return c.IPAddressPools[0].Name
}

func init() {
	registry.RegisterServiceConfig("metallb", MetalLBConfig{})
}
