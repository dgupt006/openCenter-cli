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
	"fmt"
	"strings"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

// BaseDomainDriftClass classifies the relationship between the settings-file
// cluster_defaults.base_domain and the value a cluster config renders with.
type BaseDomainDriftClass int

const (
	// BaseDomainAligned: the cluster renders with the settings domain; nothing
	// to report.
	BaseDomainAligned BaseDomainDriftClass = iota
	// BaseDomainFollowsSettings: base_domain is provenance-managed, so the
	// loader already propagated the settings value at load; the last
	// successful generation used the old value, so re-generation is pending.
	BaseDomainFollowsSettings
	// BaseDomainSettingsChanged: the settings domain changed and the cluster
	// still renders the value it was generated with (or the init placeholder).
	// The user never opted out; offer the sync remediation.
	BaseDomainSettingsChanged
	// BaseDomainPinnedDifferent: the cluster deliberately owns a different
	// domain (pinned provenance, or a cluster-side edit that pinned itself).
	// Silent by design.
	BaseDomainPinnedDifferent
)

type BaseDomainDrift struct {
	Class              BaseDomainDriftClass
	ClusterName        string
	ClusterBaseDomain  string
	SettingsBaseDomain string
	ClusterFQDN        string
	SyncedFQDN         string
	// RenderedBaseDomain is the base_domain this generation run actually
	// renders with (post-hydration, post-sync), for the success stamp.
	RenderedBaseDomain string
}

// DetectBaseDomainDrift compares the settings-file cluster_defaults.base_domain
// with the (already load-hydrated) cluster config. For a managed field,
// hydration already moved cfg.BaseDomain to the settings value, so the
// last-generation stamp distinguishes "propagated, re-generation pending" from
// "aligned". For a pinned field, the stamp distinguishes "settings changed,
// cluster never opted in" from "the user edited the cluster domain on purpose"
// (stamp absent or older than the cluster value).
func DetectBaseDomainDrift(cfg *v2.Config, settingsBaseDomain string) BaseDomainDrift {
	clusterDomain := strings.ToLower(strings.TrimSpace(cfg.OpenCenter.Cluster.BaseDomain))
	settingsDomain := strings.ToLower(strings.TrimSpace(settingsBaseDomain))
	drift := BaseDomainDrift{
		ClusterName:        cfg.ClusterName(),
		ClusterBaseDomain:  clusterDomain,
		SettingsBaseDomain: settingsDomain,
		ClusterFQDN:        cfg.OpenCenter.Cluster.ClusterFQDN,
		RenderedBaseDomain: clusterDomain,
	}
	if settingsDomain == "" {
		drift.Class = BaseDomainAligned
		return drift
	}

	stamp := strings.ToLower(strings.TrimSpace(cfg.OpenCenter.Cluster.LastGeneratedBaseDomain))

	if cfg.OpenCenter.Cluster.Managed.BaseDomainManaged() {
		if clusterDomain == settingsDomain && stamp != "" && stamp != settingsDomain {
			// Loader propagated the new settings value in memory; the last
			// successful generation still rendered the old one, so
			// re-generation is pending.
			drift.Class = BaseDomainFollowsSettings
			drift.ClusterBaseDomain = stamp
			drift.SyncedFQDN = cfg.OpenCenter.Cluster.ClusterFQDN
			return drift
		}
		drift.Class = BaseDomainAligned
		return drift
	}

	if clusterDomain == settingsDomain {
		drift.Class = BaseDomainAligned
		return drift
	}

	isPlaceholder := clusterDomain == strings.ToLower(v2.DefaultBaseDomain())
	generatedWithClusterValue := stamp != "" && stamp == clusterDomain
	if isPlaceholder || generatedWithClusterValue {
		drift.Class = BaseDomainSettingsChanged
		preview := v2.ClusterConfig{ClusterName: cfg.OpenCenter.Cluster.ClusterName, BaseDomain: settingsDomain}
		drift.SyncedFQDN = preview.DeriveClusterFQDN(cfg.OpenCenter.Meta.Region)
		if drift.SyncedFQDN == "" {
			drift.SyncedFQDN = cfg.OpenCenter.Cluster.ClusterFQDN
		}
		return drift
	}

	drift.Class = BaseDomainPinnedDifferent
	return drift
}

// Warning renders the drift as a generate warning line, or "" for classes that
// stay silent (aligned, pinned-different).
func (d BaseDomainDrift) Warning() string {
	switch d.Class {
	case BaseDomainFollowsSettings:
		return fmt.Sprintf(
			"cluster_defaults.base_domain changed to %q; cluster %q follows the settings file, so generation now renders %q (was %q). Re-run 'opencenter cluster deploy %s' to converge the live cluster.",
			d.SettingsBaseDomain, d.ClusterName, d.ClusterFQDN, d.ClusterBaseDomain, d.ClusterName,
		)
	case BaseDomainSettingsChanged:
		return fmt.Sprintf(
			"cluster_defaults.base_domain changed to %q but cluster %q still generates with %q (cluster_fqdn %s). "+
				"Run 'opencenter cluster generate %s --sync-base-domain' to adopt %q (cluster_fqdn becomes %s), or run "+
				"'opencenter cluster set %s opencenter.cluster.base_domain=%s' to keep your own value.",
			d.SettingsBaseDomain, d.ClusterName, d.ClusterBaseDomain, d.ClusterFQDN,
			d.ClusterName, d.SettingsBaseDomain, d.SyncedFQDN,
			d.ClusterName, d.SettingsBaseDomain,
		)
	default:
		return ""
	}
}
