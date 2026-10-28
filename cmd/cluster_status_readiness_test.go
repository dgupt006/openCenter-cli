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

package cmd

import (
	"strings"
	"testing"
)

// The recorded meta.status describes pipeline progress, not whether the
// cluster works. A cluster where every HelmRelease failed must not read Ready.
func TestGitOpsReadinessReadyRequiresEverything(t *testing.T) {
	healthy := &gitopsReadiness{
		Reachable: true, HelmReleaseTotal: 10, HelmReleaseReady: 10,
		KustomizationAll: 20, KustomizationRdy: 20,
		NodesTotal: 6, NodesReady: 6, NodesInitialized: 6,
	}
	if !healthy.Ready() {
		t.Error("a fully reconciled cluster must be Ready")
	}
	if !strings.HasPrefix(healthy.StatusLine(), "Ready") {
		t.Errorf("StatusLine = %q", healthy.StatusLine())
	}

	// The kit-kat shape: API reachable, nodes Ready, nothing reconciled.
	broken := &gitopsReadiness{
		Reachable: true, HelmReleaseTotal: 10, HelmReleaseReady: 0,
		KustomizationAll: 20, KustomizationRdy: 6,
		NodesTotal: 6, NodesReady: 6, NodesInitialized: 0, PodsPending: 14,
	}
	if broken.Ready() {
		t.Error("0/10 HelmReleases must never read Ready")
	}
	line := broken.StatusLine()
	for _, want := range []string{"Not ready", "0/10 HelmReleases", "6/20 Kustomizations", "14 Pending pods"} {
		if !strings.Contains(line, want) {
			t.Errorf("StatusLine %q missing %q", line, want)
		}
	}

	// Nodes Ready but not cloud-initialized still cannot schedule workloads.
	tainted := &gitopsReadiness{
		Reachable: true, NodesTotal: 6, NodesReady: 6, NodesInitialized: 0,
	}
	if tainted.Ready() {
		t.Error("tainted nodes must block Ready")
	}

	// An unreachable API is unknown, never Ready.
	unreachable := &gitopsReadiness{QueryError: "connection refused"}
	if unreachable.Ready() {
		t.Error("unreachable cluster must not be Ready")
	}
	if !strings.Contains(unreachable.StatusLine(), "connection refused") {
		t.Errorf("StatusLine should surface the query error: %q", unreachable.StatusLine())
	}
}

func TestGitOpsReadinessSummary(t *testing.T) {
	state := &gitopsReadiness{
		Reachable: true, HelmReleaseTotal: 2, HelmReleaseReady: 1,
		UnreadyReleases: []string{"observability/loki: timeout"},
	}
	summary := state.Summary()
	if summary["reachable"] != true || summary["ready"] != false {
		t.Errorf("summary = %+v", summary)
	}
	if summary["helmrelease_total"] != 2 || summary["helmrelease_ready"] != 1 {
		t.Errorf("summary counts = %+v", summary)
	}
	releases, ok := summary["helmreleases_not_ready"].([]string)
	if !ok || len(releases) != 1 || !strings.Contains(releases[0], "loki") {
		t.Errorf("summary unready releases = %v", summary["helmreleases_not_ready"])
	}
	if _, present := summary["query_error"]; present {
		t.Error("query_error should be absent when the query succeeded")
	}

	failed := (&gitopsReadiness{QueryError: "no kubeconfig"}).Summary()
	if failed["query_error"] != "no kubeconfig" {
		t.Errorf("query_error = %v", failed["query_error"])
	}
}

// A not-ready release must be reported with the reason, since "0/10 ready"
// alone forces the operator to re-derive the cause.
func TestFormatUnreadyIncludesMessage(t *testing.T) {
	item := fluxConditionItem{namespace: "openstack-ccm", name: "openstack-cloud-controller-manager", message: "Helm install failed: timeout waiting for DaemonSet"}
	got := formatUnready(item)
	if !strings.HasPrefix(got, "openstack-ccm/openstack-cloud-controller-manager: ") {
		t.Errorf("formatUnready = %q", got)
	}
	if !strings.Contains(got, "timeout waiting for DaemonSet") {
		t.Errorf("formatUnready dropped the condition message: %q", got)
	}

	// No message still yields a usable identifier.
	bare := formatUnready(fluxConditionItem{namespace: "velero", name: "velero"})
	if bare != "velero/velero" {
		t.Errorf("formatUnready = %q", bare)
	}
}

func TestTruncateCollapsesWhitespace(t *testing.T) {
	got := truncate("health  check\tfailed after\n21ms: stalled", 20)
	if !strings.HasPrefix(got, "health check failed") {
		t.Errorf("truncate = %q", got)
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("truncated output should end with an ellipsis: %q", got)
	}
	if truncate("short", 100) != "short" {
		t.Error("short strings must pass through unchanged")
	}
}

// The taint key must match what kubelet applies, or the gate never fires.
func TestCloudProviderInitTaintKeyMatchesKubernetes(t *testing.T) {
	if cloudProviderInitTaintKey != "node.cloudprovider.kubernetes.io/uninitialized" {
		t.Errorf("taint key = %q", cloudProviderInitTaintKey)
	}
}

// A cluster stalled on the cloud controller manager must be told that, not
// handed generic "run deploy" advice.
func TestGitOpsReadinessNextStepsNamesTheBlocker(t *testing.T) {
	broken := &gitopsReadiness{
		Reachable: true, HelmReleaseTotal: 10, HelmReleaseReady: 0,
		NodesTotal: 6, NodesReady: 6, NodesInitialized: 0, PodsPending: 14,
	}
	steps := strings.Join(broken.NextSteps("opensnack/kit-kat"), "\n")
	for _, want := range []string{
		"0/6 nodes",
		cloudProviderInitTaintKey,
		"flux reconcile helmrelease -n openstack-ccm openstack-cloud-controller-manager",
		"10 HelmRelease(s) are not Ready",
		"14 pod(s) are Pending",
	} {
		if !strings.Contains(steps, want) {
			t.Errorf("next steps missing %q:\n%s", want, steps)
		}
	}

	if ready := (&gitopsReadiness{Reachable: true, NodesTotal: 1, NodesReady: 1, NodesInitialized: 1}).NextSteps("c"); ready != nil {
		t.Errorf("a ready cluster should add no steps, got %v", ready)
	}

	unreachable := (&gitopsReadiness{QueryError: "connection refused"}).NextSteps("opensnack/kit-kat")
	if len(unreachable) != 1 || !strings.Contains(unreachable[0], "opensnack/kit-kat") {
		t.Errorf("unreachable next steps = %v", unreachable)
	}
}
