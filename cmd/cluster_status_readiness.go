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
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"
)

// gitopsReadiness is the live reconciliation state of a cluster's add-ons.
//
// It exists because the cluster's recorded meta.status describes how far the
// deploy pipeline got, not whether the cluster works. A cluster whose
// cloud-controller-manager could never authenticate was reported as
// "Stage: setup / Status: success" while 0 of 10 HelmReleases were Ready and
// 14 pods had been Pending for a day. Readiness has to be read off the
// cluster, which is what this does.
type gitopsReadiness struct {
	// Reachable is false when the API server could not be queried at all.
	Reachable bool
	// QueryError explains why nothing could be read.
	QueryError string

	HelmReleaseTotal int
	HelmReleaseReady int
	KustomizationAll int
	KustomizationRdy int
	NodesTotal       int
	NodesReady       int
	NodesInitialized int
	PodsPending      int
	UnreadyReleases  []string
	UnreadyKustomize []string
	BlockedNodes     []string
}

// Ready reports whether every reconciled add-on is healthy. An unreachable
// cluster is not Ready.
func (g *gitopsReadiness) Ready() bool {
	return g.Reachable && g.HelmReleaseReady == g.HelmReleaseTotal &&
		g.KustomizationRdy == g.KustomizationAll && g.PodsPending == 0 &&
		g.NodesInitialized == g.NodesTotal && g.NodesReady == g.NodesTotal
}

// StatusLine renders the aggregate verdict for the status table.
func (g *gitopsReadiness) StatusLine() string {
	if !g.Reachable {
		if g.QueryError != "" {
			return "unknown (" + g.QueryError + ")"
		}
		return "unknown"
	}

	if g.Ready() {
		return fmt.Sprintf("Ready (%d/%d HelmReleases)", g.HelmReleaseReady, g.HelmReleaseTotal)
	}
	return fmt.Sprintf("✗ Not ready (%d/%d HelmReleases, %d/%d Kustomizations, %d Pending pods)",
		g.HelmReleaseReady, g.HelmReleaseTotal, g.KustomizationRdy, g.KustomizationAll, g.PodsPending)
}

// NextSteps names the concrete blocker instead of generic lifecycle advice. A
// cluster whose nodes were never cloud-initialized is not waiting on a deploy
// step; it is waiting on the cloud controller manager, and the operator needs
// to be told that.
func (g *gitopsReadiness) NextSteps(clusterName string) []string {
	if g.Ready() {
		return nil
	}
	if !g.Reachable {
		return []string{
			fmt.Sprintf("Query the live cluster for %s: verify the cluster-owned kubeconfig and API endpoint reachability", clusterName),
		}
	}
	var steps []string
	if g.NodesInitialized < g.NodesTotal && g.NodesTotal > 0 {
		steps = append(steps,
			fmt.Sprintf("Cloud controller manager has initialized %d/%d nodes: the rest still carry the %s taint, so ordinary workloads cannot schedule",
				g.NodesInitialized, g.NodesTotal, cloudProviderInitTaintKey),
			"Reconcile it with 'flux reconcile helmrelease -n openstack-ccm openstack-cloud-controller-manager' and read its status; a Keystone auth failure here blocks the whole cluster",
		)
	}
	if g.HelmReleaseReady < g.HelmReleaseTotal {
		steps = append(steps, fmt.Sprintf("%d HelmRelease(s) are not Ready — most are downstream of the first failure, so fix the earliest one listed and re-check",
			g.HelmReleaseTotal-g.HelmReleaseReady))
	}
	if g.PodsPending > 0 {
		steps = append(steps, fmt.Sprintf("%d pod(s) are Pending; 'kubectl describe pod' on one shows the scheduling reason", g.PodsPending))
	}
	if len(steps) == 0 {
		steps = append(steps, "Inspect the not-ready HelmReleases and Kustomizations listed above")
	}
	return steps
}

// Summary renders the aggregate for structured output.
func (g *gitopsReadiness) Summary() map[string]any {
	summary := map[string]any{
		"reachable":                g.Reachable,
		"ready":                    g.Ready(),
		"status":                   g.StatusLine(),
		"helmrelease_total":        g.HelmReleaseTotal,
		"helmrelease_ready":        g.HelmReleaseReady,
		"kustomization_total":      g.KustomizationAll,
		"kustomization_ready":      g.KustomizationRdy,
		"nodes_total":              g.NodesTotal,
		"nodes_ready":              g.NodesReady,
		"nodes_initialized":        g.NodesInitialized,
		"pending_pods":             g.PodsPending,
		"helmreleases_not_ready":   g.UnreadyReleases,
		"kustomizations_not_ready": g.UnreadyKustomize,
		"nodes_not_initialized":    g.BlockedNodes,
	}
	if g.QueryError != "" {
		summary["query_error"] = g.QueryError
	}
	return summary
}

// cloudProviderInitTaintKey mirrors internal/cluster: kubelet applies it at
// registration under --cloud-provider=external and only the external CCM
// removes it, so a node still carrying it cannot run ordinary workloads.
const cloudProviderInitTaintKey = "node.cloudprovider.kubernetes.io/uninitialized"

// collectGitOpsReadiness queries the live cluster for reconciliation state.
func collectGitOpsReadiness(ctx context.Context, kubeconfigPath string) *gitopsReadiness {
	state := &gitopsReadiness{}
	timeout := 25 * time.Second
	queryCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	releases, err := queryFluxConditions(queryCtx, kubeconfigPath, "helmreleases")
	if err != nil {
		state.QueryError = err.Error()
		return state
	}
	state.Reachable = true
	state.HelmReleaseTotal = len(releases)
	for _, item := range releases {
		if item.ready {
			state.HelmReleaseReady++
			continue
		}
		state.UnreadyReleases = append(state.UnreadyReleases, formatUnready(item))
	}

	kustomizations, err := queryFluxConditions(queryCtx, kubeconfigPath, "kustomizations.kustomize.toolkit.fluxcd.io")
	if err == nil {
		state.KustomizationAll = len(kustomizations)
		for _, item := range kustomizations {
			if item.ready {
				state.KustomizationRdy++
				continue
			}
			state.UnreadyKustomize = append(state.UnreadyKustomize, formatUnready(item))
		}
	}

	if nodes, err := queryNodes(queryCtx, kubeconfigPath); err == nil {
		state.NodesTotal = len(nodes)
		for _, node := range nodes {
			if node.ready {
				state.NodesReady++
			}
			if node.providerID != "" && !node.tainted {
				state.NodesInitialized++
				continue
			}
			reason := "no providerID"
			if node.providerID != "" {
				reason = "still tainted"
			}
			if !node.ready {
				reason += ", not Ready"
			}
			state.BlockedNodes = append(state.BlockedNodes, node.name+" ("+reason+")")
		}
	}

	if pending, err := countPendingPods(queryCtx, kubeconfigPath); err == nil {
		state.PodsPending = pending
	}

	sort.Strings(state.UnreadyReleases)
	sort.Strings(state.UnreadyKustomize)
	sort.Strings(state.BlockedNodes)
	return state
}

// fluxConditionItem is one Flux object reduced to its Ready condition.
type fluxConditionItem struct {
	namespace string
	name      string
	ready     bool
	message   string
}

// queryFluxConditions reads Ready conditions for a Flux resource type. The
// jsonpath keeps the payload small; fields are joined on \t and records on
// \x1e so messages containing spaces survive.
func queryFluxConditions(ctx context.Context, kubeconfigPath, resource string) ([]fluxConditionItem, error) {
	const (
		fieldSep = "\t"
		recSep   = "\x1e"
	)
	jsonPath := fmt.Sprintf("{range .items[*]}{.metadata.namespace}%s{.metadata.name}%s{.status.conditions[?(@.type==\"Ready\")].status}%s{.status.conditions[?(@.type==\"Ready\")].message}%s%s{end}",
		fieldSep, fieldSep, fieldSep, fieldSep, recSep)
	out, err := runKubectl(ctx, kubeconfigPath, "get", resource, "-A", "-o", "jsonpath="+jsonPath)
	if err != nil {
		return nil, err
	}
	var items []fluxConditionItem
	for _, record := range strings.Split(out, recSep) {
		if strings.TrimSpace(record) == "" {
			continue
		}
		fields := strings.Split(record, fieldSep)
		if len(fields) < 3 {
			continue
		}
		item := fluxConditionItem{namespace: fields[0], name: fields[1], ready: strings.TrimSpace(fields[2]) == "True"}
		if len(fields) > 3 {
			item.message = strings.TrimSpace(fields[3])
		}
		items = append(items, item)
	}
	return items, nil
}

// formatUnready renders a not-ready Flux object with its condition message.
func formatUnready(item fluxConditionItem) string {
	if item.message == "" {
		return item.namespace + "/" + item.name
	}
	return item.namespace + "/" + item.name + ": " + truncate(item.message, 160)
}

// nodeStatus is the per-node evidence reported by status.
type nodeStatus struct {
	name       string
	ready      bool
	providerID string
	tainted    bool
}

// queryNodes reads node readiness plus the cloud-initialization evidence.
func queryNodes(ctx context.Context, kubeconfigPath string) ([]nodeStatus, error) {
	out, err := runKubectl(ctx, kubeconfigPath, "get", "nodes", "-o", "json")
	if err != nil {
		return nil, err
	}
	var payload struct {
		Items []struct {
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
			Spec struct {
				ProviderID string `json:"providerID"`
				Taints     []struct {
					Key string `json:"key"`
				} `json:"taints"`
			} `json:"spec"`
			Status struct {
				Conditions []struct {
					Type   string `json:"type"`
					Status string `json:"status"`
				} `json:"conditions"`
			} `json:"status"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		return nil, fmt.Errorf("decode nodes: %w", err)
	}
	nodes := make([]nodeStatus, 0, len(payload.Items))
	for _, item := range payload.Items {
		node := nodeStatus{name: item.Metadata.Name, providerID: strings.TrimSpace(item.Spec.ProviderID)}
		for _, condition := range item.Status.Conditions {
			if condition.Type == "Ready" && condition.Status == "True" {
				node.ready = true
			}
		}
		for _, taint := range item.Spec.Taints {
			if strings.TrimSpace(taint.Key) == cloudProviderInitTaintKey {
				node.tainted = true
			}
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}

// countPendingPods counts pods stuck in Pending, excluding completed jobs.
// This is the user-visible symptom of an uninitialized cluster.
func countPendingPods(ctx context.Context, kubeconfigPath string) (int, error) {
	out, err := runKubectl(ctx, kubeconfigPath, "get", "pods", "-A",
		"--field-selector=status.phase=Pending", "-o", "jsonpath={range .items[*]}{.metadata.name}{\"\\n\"}{end}")
	if err != nil {
		return 0, err
	}
	count := 0
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count, nil
}

// runKubectl executes kubectl against the cluster-owned kubeconfig.
func runKubectl(ctx context.Context, kubeconfigPath string, args ...string) (string, error) {
	full := append([]string{"--kubeconfig", kubeconfigPath}, args...)
	//nolint:gosec // args are constructed from fixed literals, not user input
	command := exec.CommandContext(ctx, "kubectl", full...)
	out, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("kubectl %s: %w", strings.Join(args, " "), err)
	}
	return string(out), nil
}

// truncate shortens long condition messages for table output.
func truncate(value string, limit int) string {
	value = strings.Join(strings.Fields(value), " ")
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "…"
}

// renderGitOpsReadinessText prints the aggregate plus the specific failures,
// so the operator does not have to re-derive what is broken.
func renderGitOpsReadinessText(writer interface{ Write([]byte) (int, error) }, state *gitopsReadiness) {
	fmt.Fprintf(writer, "\nGitOps Readiness:\n")
	fmt.Fprintf(writer, "  Status:            %s\n", state.StatusLine())
	if !state.Reachable {
		return
	}
	fmt.Fprintf(writer, "  Nodes:             %d/%d Ready, %d/%d cloud-initialized\n",
		state.NodesReady, state.NodesTotal, state.NodesInitialized, state.NodesTotal)
	if len(state.BlockedNodes) > 0 {
		for _, node := range state.BlockedNodes {
			fmt.Fprintf(writer, "    - %s\n", node)
		}
	}
	if len(state.UnreadyReleases) > 0 {
		fmt.Fprintf(writer, "  HelmReleases not ready:\n")
		for _, release := range state.UnreadyReleases {
			fmt.Fprintf(writer, "    - %s\n", release)
		}
	}
	if len(state.UnreadyKustomize) > 0 {
		fmt.Fprintf(writer, "  Kustomizations not ready:\n")
		for _, kustomization := range state.UnreadyKustomize {
			fmt.Fprintf(writer, "    - %s\n", kustomization)
		}
	}
	if state.PodsPending > 0 {
		fmt.Fprintf(writer, "  Pending pods:      %d\n", state.PodsPending)
	}
}
