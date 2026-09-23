---
id: cluster-deploy-openstack
title: "cluster deploy — OpenStack Provider"
sidebar_label: cluster deploy — OpenStack
description: How cluster deploy works end-to-end for the OpenStack provider, including every bootstrap step and why the Kubespray steps are currently disabled.
doc_type: explanation
audience: "developers"
tags: [openstack, deploy, bootstrap, kubespray, opentofu]
---
# cluster deploy — OpenStack Provider

**Purpose:** For developers, explains how `opencenter cluster deploy` works end-to-end for the OpenStack provider: the CLI pre-flight, the bootstrap step sequence, and the current (not the originally-designed) way Kubespray gets run.

The entry point is `cmd/cluster_deploy.go`. Orchestration lives in `internal/cluster/bootstrap_service.go`. The OpenStack-specific step sequence is built by `openstackBootstrapProvider.BuildSteps` in `internal/cluster/bootstrap_provider_infra.go` — **not** a separate `openstack_bootstrap_provider.go` file (that filename does not exist in the current tree; don't trust older references to it).

## Overview

1. **CLI pre-flight** (`cmd/cluster_deploy.go`) — resolve the cluster name, load and validate the config, check provider availability, acquire a deploy lock, verify the GitOps working tree is clean, verify the Git remote matches config, mark cluster status `running`.
2. **Bootstrap orchestration** (`internal/cluster/bootstrap_service.go`) — resolve cluster paths, open the bootstrap log, load any saved resume state, then hand off to the provider's `BuildSteps`.
3. **Step execution** (`internal/cluster/bootstrap_provider_infra.go`) — the OpenStack step sequence below.
4. **Readiness polling** — wait for the Kubernetes API to respond before declaring success.

## The current OpenStack step sequence

`openstackBootstrapProvider.BuildSteps` builds these steps, in order:

1. **`preflight`** — validates OpenStack credentials via `internal/credentials.Extractor.ExtractOpenStack()` plus `openstackprovider.PreflightOpenStack(authURL)`. Rejects empty credentials, and rejects `application_credential_id`/`application_credential_secret` still set to the `CHANGEME` placeholder.
2. **`opentofu-init`** — runs `opentofu init` in the cluster's infrastructure directory (`<gitDir>/infrastructure/clusters/<cluster>`), with `OS_*` credential env vars merged in via `credentials.OpenStackCredentials.ToEnvMap()` (`OS_AUTH_URL`, `OS_REGION_NAME`, `OS_APPLICATION_CREDENTIAL_ID`, `OS_APPLICATION_CREDENTIAL_SECRET`, `OS_USERNAME`/`OS_PASSWORD` as a fallback, plus fixed `OS_INTERFACE=public` and `OS_IDENTITY_API_VERSION=3`) and `KUBECONFIG` set to the resolved cluster kubeconfig path.
3. **`opentofu-apply`** — runs `opentofu apply -auto-approve` with the same environment.
4. **`openstack-normalize-kubeconfig`** — finds whichever kubeconfig the tooling actually wrote (`opts.KubeconfigPath`, then `kubeconfig.yaml`, `kubeconfig`, or `kube_config_cluster.yml` inside the cluster directory, in that order), rewrites any `127.0.0.1`/`localhost`/`[::1]` server host to the cluster's API endpoint IP (`infrastructure.k8s_api_ip` if set, else the VRRP VIP when `vrrp_enabled: true`), and writes the result to `opts.KubeconfigPath` with mode `0600`.
5. **Network plugin install step** (`buildNetworkPluginInstallStep`) — installs the enabled CNI (Calico/Cilium/Kube-OVN) so the cluster is network-ready.
6. **Flux bootstrap step, SOPS age secret step, Grafana admin secret step** — added *only* when `gitops.auth.token` is configured with a non-empty provider and a real (non-placeholder) repository URL. These run after the CNI so FluxCD's source-controller has a working network when it starts reconciling. No credential Secret is created for the shared `openCenter-gitops-base` repository itself — it's public, so its `GitRepository` source is rendered anonymously.

There is deliberately **no automatic git push** — the operator commits and pushes the generated GitOps repository manually after `cluster deploy` completes.

### Why there's no separate Kubespray step anymore

An earlier design added three more steps — `kubespray-venv-create`, `kubespray-pip-install`, `kubespray-ansible-playbook` — to run Kubespray's Ansible playbook explicitly. That code still exists (`buildKubespraySteps`) but is **commented out** in `BuildSteps`, with this reasoning left in the source:

> The OpenTofu module for this provider already embeds a `null_resource.run_kubespray` with a `local-exec` provisioner that runs the full Ansible/Kubespray playbook as part of `opentofu-apply`. Appending these steps caused Ansible to run a second time against a cluster that was already provisioned, wasting ~1h of deploy time.

So today, for `deployment.method: kubespray`, the Kubespray run happens **inside** `opentofu-apply` (step 2 above), not as its own step. The tracked long-term fix is to remove the `null_resource.run_kubespray` local-exec from the OpenTofu templates so OpenTofu only provisions infrastructure and a dedicated step owns the Ansible run exclusively — if you're touching bootstrap sequencing, check whether that fix has landed before trusting a "kubespray step" description anywhere else.

## State and resume

Bootstrap step results are persisted to `state.json` after each step runs (`internal/cluster/bootstrap_service.go` / `bootstrap_runtime.go`):

```
<state dir>/bootstrap/<org>/<cluster>/state.json     — resume state
<state dir>/logs/bootstrap/<org>/<cluster>/bootstrap-<timestamp>.log  — full command output
```

Re-running `cluster deploy` reads saved state and skips steps already marked `success`. Relevant flags: `--restart` (ignore saved state, rerun everything), `--step <id>` (run exactly one step), `--from-step <id>` (run from a given step onward).

## Files involved

| File | Role |
| --- | --- |
| `cmd/cluster_deploy.go` | Entry point: flags, lock acquisition, git hygiene checks, status updates. |
| `internal/cluster/bootstrap_service.go` | Orchestrator: path resolution, config load, step execution, resume-state handling, readiness polling. |
| `internal/cluster/bootstrap_provider.go` | `lifecycleBootstrapProvider` interface and the `bootstrapStep` type every provider builds. |
| `internal/cluster/bootstrap_provider_infra.go` | The OpenStack (and shared infra-provider) step sequence described above. |
| `internal/cluster/bootstrap_plan.go` | `BootstrapPlan` types and the `--dry-run` plan renderer. |
| `internal/cluster/bootstrap_runtime.go` | Log-file and state-file path resolution. |
| `internal/credentials/extractor.go`, `internal/credentials/openstack.go` | Pull OpenStack credentials out of `v2.Config` and turn them into `OS_*` env vars. |
| `internal/cloud/openstack/preflight.go` | CLI availability + `auth_url` sanity checks. |

## Related

- [Cluster Init Details](cluster-init-details.md) — how the config bootstrap reads from is created.
- [Renderer Contract](rendering-contract.md) — how the GitOps repository bootstrap deploys into is generated.
- `internal/cluster/magnum_bootstrap_provider.go` — contrast with a provider that has no OpenTofu/Kubespray step at all (see [Adding New Infrastructure Providers](adding-providers.md)).
