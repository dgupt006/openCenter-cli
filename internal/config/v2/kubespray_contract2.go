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

import "strings"

// kubesprayModuleSourceNeedsContract2Upgrade reports whether a Kubespray
// module source pins the gitops-base kubespray module at a ref older than
// lifecycle contract v2 (inventory_output_dir: the inventory renders once,
// into the CLI state zone). A pre-contract-2 ref (2026.03-rc3 and older)
// renders the inventory inside the GitOps worktree, so `cluster deploy` fails
// in kubespray-prepare with "inventory rendered outside the cluster state
// zone".
//
// An empty source needs no upgrade: templates and the load pipeline backfill
// DefaultKubesprayModuleSourceURL. Sources outside the gitops-base kubespray
// module path are deliberate user pins and are never flagged.
func kubesprayModuleSourceNeedsContract2Upgrade(source string) bool {
	trimmed := strings.TrimSpace(source)
	if trimmed == "" {
		return false
	}
	normalized := strings.TrimSuffix(trimmed, ".git")
	if !strings.Contains(normalized, "//iac/provider/kubespray") ||
		!strings.Contains(normalized, "openCenter-gitops-base") {
		return false
	}
	ref := ""
	if idx := strings.Index(normalized, "?ref="); idx >= 0 {
		ref = normalized[idx+len("?ref="):]
		if amp := strings.Index(ref, "&"); amp >= 0 {
			ref = ref[:amp]
		}
	}
	return ref != "" && ref != kubesprayModuleTag
}

// RequiresKubesprayModuleContract2Upgrade reports whether any Kubespray module
// slot (deployment.kubespray.kubespray_cluster for the baremetal/vmware
// templates, deployment.kubespray.modules.kubespray for the openstack/default
// template) still pins a pre-contract-2 gitops-base ref. The CLI deploy path
// always renders in lifecycle mode, so every slot must resolve contract v2.
func RequiresKubesprayModuleContract2Upgrade(cfg *Config) bool {
	if cfg == nil || cfg.Deployment.Kubespray == nil ||
		!strings.EqualFold(strings.TrimSpace(cfg.Deployment.Method), "kubespray") {
		return false
	}
	if kubesprayModuleSourceNeedsContract2Upgrade(cfg.Deployment.Kubespray.KubesprayCluster.Source) {
		return true
	}
	for _, module := range cfg.Deployment.Kubespray.Modules {
		if kubesprayModuleSourceNeedsContract2Upgrade(module.Source) {
			return true
		}
	}
	return false
}
