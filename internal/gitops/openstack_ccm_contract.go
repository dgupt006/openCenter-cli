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
	"fmt"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

// openstackCCMBaseKustomizationPatchesRenderer emits the inline patch on the
// Flux base Kustomization for the OpenStack cloud-controller-manager.
//
// The upstream base ships install.remediation with retries plus uninstall-on-
// last-failure. That default is wrong for the one component whose job is to
// make the cluster schedulable. Kubespray is configured with
// cloud_provider: external and external_cloud_provider: manual, which means it
// deliberately deploys no CCM: every node boots carrying the
// node.cloudprovider.kubernetes.io/uninitialized:NoSchedule taint that kubelet
// applied at registration, and the CCM is the only component that ever removes
// it. So when a CCM install times out — a bad auth_url, a slow first pull, a
// transient Keystone error — Flux uninstalls the release, and the cluster is
// left with a taint nothing can clear, no CCM pods left to inspect, and no
// path back to Ready without human intervention. Observed on a real cluster:
// 14 pods stuck Pending for 24 hours with 0 of 10 HelmReleases Ready, while
// deploy reported success.
//
// The patch keeps the retries so a transient failure still recovers, but drops
// the uninstall so a persistent failure leaves a stalled release plus its pods
// and events behind — visible, diagnosable, and one `flux reconcile` away from
// working once the cause is fixed.
func openstackCCMBaseKustomizationPatchesRenderer(cfg v2.Config) (string, error) {
	return fmt.Sprintf(`  patches:
    - patch: |-
        apiVersion: helm.toolkit.fluxcd.io/v2
        kind: HelmRelease
        metadata:
          name: %[1]s
        spec:
          install:
            remediation:
              retries: 5
              remediateLastFailure: false
          upgrade:
            remediation:
              # Leave a failed upgrade in place: rolling back the CCM re-taints
              # nothing but removes the evidence needed to diagnose it.
              retries: 0
              remediateLastFailure: false
      target:
        group: helm.toolkit.fluxcd.io
        version: v2
        kind: HelmRelease
        name: %[1]s
`, openstackCCMHelmReleaseName), nil
}

// openstackCCMHelmReleaseName is the metadata.name of the CCM HelmRelease in
// the upstream base repository. It is independent of the opencenter service
// key, which is why it is named separately.
const openstackCCMHelmReleaseName = "openstack-cloud-controller-manager"
