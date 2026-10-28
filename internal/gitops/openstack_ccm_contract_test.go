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

package gitops

import (
	"strings"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

// The upstream CCM base ships install remediation that uninstalls the release
// on last failure. With external_cloud_provider: manual the CCM is the only
// component that clears the cloud-provider-uninitialized taint, so uninstalling
// it leaves the cluster with no path back to schedulable. The rendered patch
// must therefore keep the release in place.
func TestOpenstackCCMPatchForbidsUninstallRemediation(t *testing.T) {
	patch, err := openstackCCMBaseKustomizationPatchesRenderer(v2.Config{})
	if err != nil {
		t.Fatalf("renderer: %v", err)
	}

	if !strings.Contains(patch, "remediateLastFailure: false") {
		t.Error("patch must disable remediateLastFailure so a failed CCM install is not uninstalled")
	}
	if strings.Contains(patch, "remediateLastFailure: true") {
		t.Error("patch must not enable uninstall remediation")
	}
	// Retries are kept: a transient failure should still self-heal.
	if !strings.Contains(patch, "retries: 5") {
		t.Error("patch must keep install retries")
	}
	if !strings.Contains(patch, "kind: HelmRelease") || !strings.Contains(patch, openstackCCMHelmReleaseName) {
		t.Error("patch must target the CCM HelmRelease by name")
	}
	// The patch block must be valid for the base Kustomization: two-space
	// indented `patches:` with a literal block scalar.
	if !strings.HasPrefix(patch, "  patches:\n    - patch: |-\n        apiVersion: helm.toolkit.fluxcd.io/v2") {
		t.Errorf("unexpected patch shape:\n%s", patch)
	}
}

// The CCM catalog entry must wire the remediation patch in and stay
// bootstrap-priority: it cannot wait on the sources gate, because DNS — which
// the sources gate needs to fetch repos — only works once the CCM clears the
// cloud-provider-uninitialized taint.
func TestOpenstackCCMCatalogWiresRemediationPatch(t *testing.T) {
	spec, ok := newBuiltInRenderCatalog().Lookup("openstack-ccm")
	if !ok {
		t.Fatal("openstack-ccm missing from the render catalog")
	}
	if spec.BaseKustomizationPatchesRenderer == nil {
		t.Fatal("openstack-ccm must render the remediation patch")
	}
	if !spec.SkipSourcesDependency {
		t.Error("openstack-ccm must skip the sources dependency or it deadlocks on DNS")
	}
}
