---
id: cluster-lifecycle-map
title: "Explain the Cluster Lifecycle"
sidebar_label: Cluster Lifecycle
description: Explains how cluster configuration moves through initialization, validation, GitOps generation, resumable bootstrap, operations, and destruction.
doc_type: explanation
audience: "contributors, maintainers, operators"
tags: [clusters, lifecycle, bootstrap, gitops, providers]
---
# Cluster lifecycle

`internal/cluster` owns lifecycle orchestration. `cmd/` selects a service and translates flags; configuration, rendering, provider, and operational packages perform the work.

## Lifecycle flow

```text
cluster init
  -> cluster configure (optional guided changes)
  -> cluster validate / doctor
  -> cluster generate
  -> cluster deploy
  -> cluster status, drift, backup, pool, service operations
  -> cluster destroy
```

Existing repositories can enter through `cluster import scan`, `report`, and `apply`; see [Import, operations, and resilience](import-operations-and-resilience.md).

## Service ownership

| Service or area | Entry point | Responsibility |
|---|---|---|
| Init | `internal/cluster/init_service.go` | Create organization-aware cluster paths, defaults, and key material |
| Configure | `internal/cluster/configure_service.go` and `orchestration/` | Guided provider and capability changes, then persist managed files |
| Validate | `internal/cluster/validate_service.go` | Load config and produce readiness, provider, service, and GitOps reports |
| Generate | `internal/cluster/setup_service.go` | Run the live GitOps generation path described below |
| OpenStack provider/storage operations | `cmd/cluster_provider_openstack.go`, `cmd/cluster_service_storage.go`, and `internal/cluster/{provider,storage}/openstack/` | Plan/apply typed provider changes or provision one service's storage; separate from bootstrap and generation |
| Bootstrap | `internal/cluster/bootstrap_service.go` | Execute provider-specific resumable infrastructure and cluster steps |
| Destroy | `internal/cluster/destroy_service.go` | Execute provider-specific teardown steps |
| Pool/service commands | `cmd/cluster_pool.go`, `cmd/cluster_service.go` | Mutate or inspect declared cluster topology and services |

## Generate path

`SetupService.Setup` resolves the cluster, loads configuration, checks schema version `2.0`, validates setup inputs, and then calls the GitOps package directly:

1. `gitops.CopyBase` copies the base repository structure.
2. `gitops.RenderClusterAppsWithEncryption` renders application descriptors/catalog actions and encrypts temporary override values before promotion.
3. `gitops.RenderInfrastructureCluster` renders provider-selected infrastructure assets.
4. `gitops.RenderClusterFluxBridge` renders the per-cluster Flux bridge.
5. For non-Kind providers, `tofu.Provision` performs the optional OpenTofu provisioning step in the generate path.
6. Generated files are counted and manifests are validated; validation warnings are returned in the result.

`PipelineGenerator` remains a supporting `internal/gitops` API for staged generation abstractions. It is not the live top-level path used by `SetupService`.

See [GitOps engine](gitops-engine.md) and [Rendering ownership and secret artifacts](rendering-ownership-and-secret-artifacts.md).

## OpenStack provider and storage operations

The lifecycle flow exposes two explicit OpenStack operation families outside bootstrap and generation. `cluster provider openstack plan/apply` performs read-only discovery and persists only validated typed provider changes. `cluster service storage plan/apply` provisions exactly one supported service's Swift or S3 storage mapping and sequences remote actions, typed persistence, credential reuse/rotation, and recovery. See [OpenStack provider and storage operations](openstack-provider-storage-operations.md).

## Bootstrap path

`BootstrapService` loads the validated config, resolves runtime log/state paths, selects a lifecycle provider, builds ordered steps, and persists step status in `bootstrap-state.json`. `--step` selects one step; `--from-step` resumes from a step boundary. The result includes a dry-run plan, completed/failed step IDs, endpoint, log path, and resume-state path.

| Provider path | Bootstrap implementation | Main boundary |
|---|---|---|
| OpenStack | `openstackBootstrapProvider` in `bootstrap_provider_infra.go` | OpenTofu init/apply, cluster provisioning, network plugin, Flux, live secrets |
| VMware | Shared `openstackBootstrapProvider` with vSphere validation/environment | OpenTofu/Kubespray-style infrastructure path and Flux |
| Baremetal | Shared provider implementation with static-node validation and no cloud credentials | Infrastructure path and Kubernetes initialization without OpenStack/vSphere environment |
| Kind | `kindBootstrapProvider` in `kind_bootstrap_provider.go` | Kind create/readiness, local Flux/Gitea integration, live secret reconciliation |

Provider routing and capability distinctions are detailed in [Providers](providers.md). Resumable state and distributed operation locks are supported by [Import, operations, and resilience](import-operations-and-resilience.md).

## Destroy and day-2 boundaries

Destroy uses `lifecycleDestroyProvider` implementations; OpenStack has an OpenTofu destroy path and Kind delegates to its cloud/kind lifecycle provider. Drift, backup, import, and lock commands use packages outside the lifecycle service and must not be folded into rendering or config loading.

## Cross-module boundaries

- `internal/config/v2` is the input and persistence contract for lifecycle services and the OpenStack provider/storage operations. Provider and storage paths operate on typed v2 values; storage apply adds explicit remote-action and recovery sequencing.
- `internal/core/paths` resolves organization/cluster identifiers and runtime paths.
- `internal/core/validation` provides shared validation primitives.
- `internal/security` supplies sanitized command execution and audit components.
- `internal/resilience` protects mutating operations with locks, retries, and circuit breakers.

## Related maps

- [Config system](config-system.md)
- [OpenStack provider and storage operations](openstack-provider-storage-operations.md)
- [GitOps engine](gitops-engine.md)
- [Providers](providers.md)
- [Secrets management](secrets-management.md)
