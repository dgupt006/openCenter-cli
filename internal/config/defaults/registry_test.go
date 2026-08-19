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

package defaults

import (
	"testing"
)

// TestRegistry_GetDefaults_CaseInsensitive verifies that GetDefaults performs
// case-insensitive lookups for both provider and region (OCTR-628).
func TestRegistry_GetDefaults_CaseInsensitive(t *testing.T) {
	registry := NewRegistry()

	tests := []struct {
		name     string
		provider string
		region   string
	}{
		{name: "lowercase provider and region", provider: "openstack", region: "dfw3"},
		{name: "uppercase region", provider: "openstack", region: "DFW3"},
		{name: "mixed case region", provider: "openstack", region: "Dfw3"},
		{name: "uppercase provider", provider: "OPENSTACK", region: "dfw3"},
		{name: "mixed case provider", provider: "OpenStack", region: "dfw3"},
		{name: "uppercase provider and region", provider: "OPENSTACK", region: "DFW3"},
		{name: "aws uppercase region", provider: "aws", region: "US-EAST-1"},
		{name: "aws mixed case provider", provider: "AWS", region: "us-east-1"},
		{name: "gcp mixed case", provider: "GCP", region: "US-CENTRAL1"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			defaults, err := registry.GetDefaults(tc.provider, tc.region)
			if err != nil {
				t.Fatalf("GetDefaults(%q, %q) returned unexpected error: %v", tc.provider, tc.region, err)
			}
			if defaults == nil {
				t.Fatalf("GetDefaults(%q, %q) returned nil defaults", tc.provider, tc.region)
			}

			// Verify it returns the same data as the canonical lowercase lookup
			canonical, _ := registry.GetDefaults("openstack", "dfw3")
			if tc.provider == "openstack" || tc.provider == "OPENSTACK" || tc.provider == "OpenStack" {
				if tc.region == "dfw3" || tc.region == "DFW3" || tc.region == "Dfw3" {
					if defaults.GetImageID("24") != canonical.GetImageID("24") {
						t.Errorf("case-insensitive lookup returned different image ID: got %q, want %q",
							defaults.GetImageID("24"), canonical.GetImageID("24"))
					}
				}
			}
		})
	}
}

// TestRegistry_RegisterDefaults_CaseInsensitive verifies that RegisterDefaults
// stores keys in a case-insensitive manner.
func TestRegistry_RegisterDefaults_CaseInsensitive(t *testing.T) {
	registry := NewRegistry()

	// Register with mixed case
	mock := &mockProviderDefaults{imageID: "test-image-mixed"}
	registry.RegisterDefaults("MyProvider", "MyRegion", mock)

	// Should be retrievable with any case
	tests := []struct {
		provider string
		region   string
	}{
		{"myprovider", "myregion"},
		{"MYPROVIDER", "MYREGION"},
		{"MyProvider", "MyRegion"},
		{"mYpRoViDeR", "mYrEgIoN"},
	}

	for _, tc := range tests {
		defaults, err := registry.GetDefaults(tc.provider, tc.region)
		if err != nil {
			t.Errorf("GetDefaults(%q, %q) failed after mixed-case registration: %v", tc.provider, tc.region, err)
			continue
		}
		if defaults.GetImageID("24") != "test-image-mixed" {
			t.Errorf("GetDefaults(%q, %q) returned wrong image ID: got %q, want %q",
				tc.provider, tc.region, defaults.GetImageID("24"), "test-image-mixed")
		}
	}
}

// TestRegistry_ListRegions_CaseInsensitive verifies that ListRegions performs
// a case-insensitive provider lookup.
func TestRegistry_ListRegions_CaseInsensitive(t *testing.T) {
	registry := NewRegistry()

	lowercaseRegions := registry.ListRegions("openstack")
	if len(lowercaseRegions) == 0 {
		t.Fatal("ListRegions(\"openstack\") returned no regions")
	}

	tests := []string{"OPENSTACK", "OpenStack", "Openstack", "openStack"}
	for _, provider := range tests {
		regions := registry.ListRegions(provider)
		if len(regions) != len(lowercaseRegions) {
			t.Errorf("ListRegions(%q) returned %d regions, want %d", provider, len(regions), len(lowercaseRegions))
			continue
		}
		for i, r := range regions {
			if r != lowercaseRegions[i] {
				t.Errorf("ListRegions(%q)[%d] = %q, want %q", provider, i, r, lowercaseRegions[i])
			}
		}
	}
}

// mockProviderDefaults is a minimal mock for testing registration.
type mockProviderDefaults struct {
	imageID string
}

func (m *mockProviderDefaults) GetImageID(_ string) string       { return m.imageID }
func (m *mockProviderDefaults) GetAvailabilityZones() []string   { return []string{"az1"} }
func (m *mockProviderDefaults) GetNTPServers() []string          { return []string{"ntp.example.com"} }
func (m *mockProviderDefaults) GetDNSNameservers() []string      { return []string{"8.8.8.8"} }
func (m *mockProviderDefaults) GetDefaultStorageClass() string   { return "standard" }
func (m *mockProviderDefaults) GetDefaultFlavors() FlavorDefaults {
	return FlavorDefaults{Bastion: "small", Master: "medium", Worker: "large", WorkerWindows: "xlarge"}
}
