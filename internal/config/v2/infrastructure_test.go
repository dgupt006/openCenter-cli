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

package v2

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestNetworkingConfig_VIPInterface_YAMLRoundTrip(t *testing.T) {
	tests := []struct {
		name         string
		vipInterface string
		description  string
	}{
		{
			name:         "eth0 interface",
			vipInterface: "eth0",
			description:  "common physical interface",
		},
		{
			name:         "ens3 interface",
			vipInterface: "ens3",
			description:  "predictable network interface name",
		},
		{
			name:         "bond0 interface",
			vipInterface: "bond0",
			description:  "bonded interface",
		},
		{
			name:         "empty interface",
			vipInterface: "",
			description:  "omitted when empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := NetworkingConfig{
				SubnetNodes:          "10.2.128.0/22",
				AllocationPoolStart:  "10.2.128.10",
				AllocationPoolEnd:    "10.2.131.250",
				Gateway:              "10.2.128.1",
				VRRPEnabled:          true,
				VRRPIP:               "10.2.128.5",
				LoadbalancerProvider: "metallb",
				DNSZoneName:          "example.com.",
				DNSNameservers:       []string{"8.8.8.8"},
				NTPServers:           []string{"pool.ntp.org"},
				VIPInterface:         tt.vipInterface,
			}

			// Marshal to YAML
			data, err := yaml.Marshal(&original)
			if err != nil {
				t.Fatalf("marshal error: %v", err)
			}

			// If vip_interface is empty, it should not appear in YAML (omitempty)
			if tt.vipInterface == "" {
				if contains(string(data), "vip_interface") {
					t.Error("expected vip_interface to be omitted from YAML when empty")
				}
				return
			}

			// If vip_interface is set, it should appear in YAML
			if !contains(string(data), "vip_interface") {
				t.Errorf("expected vip_interface to be present in YAML, got:\n%s", string(data))
			}

			// Unmarshal back
			var decoded NetworkingConfig
			if err := yaml.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("unmarshal error: %v", err)
			}

			if decoded.VIPInterface != original.VIPInterface {
				t.Errorf("VIPInterface mismatch: got %q, want %q", decoded.VIPInterface, original.VIPInterface)
			}
		})
	}
}

func TestNetworkingConfig_VIPInterface_UnmarshalFromYAML(t *testing.T) {
	yamlData := `
subnet_nodes: "10.2.128.0/22"
allocation_pool_start: "10.2.128.10"
allocation_pool_end: "10.2.131.250"
vrrp_enabled: true
vrrp_ip: "10.2.128.5"
vip_interface: "ens3"
loadbalancer_provider: "metallb"
dns_zone_name: "example.com."
dns_nameservers:
  - "8.8.8.8"
ntp_servers:
  - "pool.ntp.org"
`

	var cfg NetworkingConfig
	if err := yaml.Unmarshal([]byte(yamlData), &cfg); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if cfg.VIPInterface != "ens3" {
		t.Errorf("expected VIPInterface = %q, got %q", "ens3", cfg.VIPInterface)
	}
	if !cfg.VRRPEnabled {
		t.Error("expected VRRPEnabled = true")
	}
	if cfg.VRRPIP != "10.2.128.5" {
		t.Errorf("expected VRRPIP = %q, got %q", "10.2.128.5", cfg.VRRPIP)
	}
}

func TestNetworkingConfig_VIPInterface_OmittedFromYAML(t *testing.T) {
	yamlData := `
subnet_nodes: "10.2.128.0/22"
allocation_pool_start: "10.2.128.10"
allocation_pool_end: "10.2.131.250"
vrrp_enabled: true
vrrp_ip: "10.2.128.5"
loadbalancer_provider: "metallb"
dns_zone_name: "example.com."
dns_nameservers:
  - "8.8.8.8"
ntp_servers:
  - "pool.ntp.org"
`

	var cfg NetworkingConfig
	if err := yaml.Unmarshal([]byte(yamlData), &cfg); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if cfg.VIPInterface != "" {
		t.Errorf("expected VIPInterface to be empty when not in YAML, got %q", cfg.VIPInterface)
	}
}

// contains is a simple helper to check if a string contains a substring.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
