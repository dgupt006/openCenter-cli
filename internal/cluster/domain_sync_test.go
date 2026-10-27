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

package cluster

import (
	"strings"
	"testing"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

func driftTestConfig(mutate func(*v2.ClusterConfig)) *v2.Config {
	cfg := &v2.Config{}
	cfg.OpenCenter.Meta.Name = "acme"
	cfg.OpenCenter.Meta.Region = "sjc3"
	cfg.OpenCenter.Cluster = v2.ClusterConfig{
		ClusterName: "acme",
		BaseDomain:  "k8s.example.com",
		ClusterFQDN: "acme.sjc3.k8s.example.com",
	}
	cfg.OpenCenter.Cluster.Managed.PinBaseDomain()
	if mutate != nil {
		mutate(&cfg.OpenCenter.Cluster)
	}
	return cfg
}

func TestDetectBaseDomainDriftAligned(t *testing.T) {
	cfg := driftTestConfig(func(c *v2.ClusterConfig) {
		c.LastGeneratedBaseDomain = "k8s.example.com"
	})
	drift := DetectBaseDomainDrift(cfg, "k8s.example.com")
	if drift.Class != BaseDomainAligned {
		t.Fatalf("class = %v, want aligned", drift.Class)
	}
	if drift.Warning() != "" {
		t.Fatalf("aligned drift must stay silent, got %q", drift.Warning())
	}
	if drift.RenderedBaseDomain != "k8s.example.com" {
		t.Fatalf("rendered = %q", drift.RenderedBaseDomain)
	}
}

func TestDetectBaseDomainDriftNoSettingsValue(t *testing.T) {
	cfg := driftTestConfig(nil)
	drift := DetectBaseDomainDrift(cfg, "")
	if drift.Class != BaseDomainAligned || drift.Warning() != "" {
		t.Fatalf("no settings value must stay silent, got class=%v warning=%q", drift.Class, drift.Warning())
	}
}

func TestDetectBaseDomainDriftSettingsChangedAfterGeneration(t *testing.T) {
	// The cluster generated successfully with k8s.example.com; the settings
	// file was changed afterwards. The user never opted out: warn and offer
	// the sync remediation with the previewed FQDN.
	cfg := driftTestConfig(func(c *v2.ClusterConfig) {
		c.LastGeneratedBaseDomain = "k8s.example.com"
	})
	drift := DetectBaseDomainDrift(cfg, "new.example.org")
	if drift.Class != BaseDomainSettingsChanged {
		t.Fatalf("class = %v, want settings-changed", drift.Class)
	}
	warning := drift.Warning()
	for _, want := range []string{"new.example.org", "--sync-base-domain", "acme.sjc3.new.example.org"} {
		if !strings.Contains(warning, want) {
			t.Fatalf("warning %q missing %q", warning, want)
		}
	}
}

func TestDetectBaseDomainDriftPlaceholderWarns(t *testing.T) {
	// A cluster that never left the init placeholder domain: settings has a
	// real domain but provenance stayed pinned (e.g. legacy config); warn.
	cfg := driftTestConfig(func(c *v2.ClusterConfig) {
		c.BaseDomain = v2.DefaultBaseDomain()
		c.ClusterFQDN = "acme.sjc3." + v2.DefaultBaseDomain()
	})
	drift := DetectBaseDomainDrift(cfg, "k8s.example.com")
	if drift.Class != BaseDomainSettingsChanged {
		t.Fatalf("class = %v, want settings-changed for placeholder", drift.Class)
	}
	if drift.Warning() == "" {
		t.Fatal("placeholder cluster must warn")
	}
}

func TestDetectBaseDomainDriftUserEditedStaysSilent(t *testing.T) {
	// No generation stamp matches the cluster value and the domain is not the
	// placeholder: the user set it deliberately. Settings must not nag.
	cfg := driftTestConfig(func(c *v2.ClusterConfig) {
		c.BaseDomain = "customer.example.net"
		c.ClusterFQDN = "acme.sjc3.customer.example.net"
	})
	drift := DetectBaseDomainDrift(cfg, "k8s.example.com")
	if drift.Class != BaseDomainPinnedDifferent {
		t.Fatalf("class = %v, want pinned-different", drift.Class)
	}
	if drift.Warning() != "" {
		t.Fatalf("pinned cluster must stay silent, got %q", drift.Warning())
	}
}

func TestDetectBaseDomainDriftManagedFollowsSettings(t *testing.T) {
	// Managed field: the loader propagated the new settings value into
	// cfg.BaseDomain, but the last successful generation stamped the old one.
	cfg := driftTestConfig(func(c *v2.ClusterConfig) {
		c.Managed.DefaultBaseDomain()
		c.BaseDomain = "new.example.org"
		c.ClusterFQDN = "acme.sjc3.new.example.org"
		c.LastGeneratedBaseDomain = "k8s.example.com"
	})
	drift := DetectBaseDomainDrift(cfg, "new.example.org")
	if drift.Class != BaseDomainFollowsSettings {
		t.Fatalf("class = %v, want follows-settings", drift.Class)
	}
	warning := drift.Warning()
	for _, want := range []string{"new.example.org", "k8s.example.com", "acme.sjc3.new.example.org"} {
		if !strings.Contains(warning, want) {
			t.Fatalf("warning %q missing %q", warning, want)
		}
	}
	if drift.RenderedBaseDomain != "new.example.org" {
		t.Fatalf("rendered = %q, want the propagated value for stamping", drift.RenderedBaseDomain)
	}
}

func TestDetectBaseDomainDriftManagedAlignedAfterRegeneration(t *testing.T) {
	// After the follow-settings generation succeeded and stamped the new
	// value, the next run must be silent.
	cfg := driftTestConfig(func(c *v2.ClusterConfig) {
		c.Managed.DefaultBaseDomain()
		c.BaseDomain = "new.example.org"
		c.ClusterFQDN = "acme.sjc3.new.example.org"
		c.LastGeneratedBaseDomain = "new.example.org"
	})
	drift := DetectBaseDomainDrift(cfg, "new.example.org")
	if drift.Class != BaseDomainAligned || drift.Warning() != "" {
		t.Fatalf("steady-state managed cluster must be silent, got class=%v warning=%q", drift.Class, drift.Warning())
	}
}
