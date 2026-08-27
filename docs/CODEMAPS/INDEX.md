---
id: codemaps-index
title: "openCenter CLI Architecture Maps"
sidebar_label: Architecture Maps
description: Explains the openCenter CLI runtime entry points, package ownership, control flow, and navigation across the durable architecture maps.
doc_type: explanation
audience: "contributors, maintainers, code-oriented agents"
tags: [architecture, codemaps, cli, gitops, packages]
---
# openCenter CLI architecture maps

These maps describe durable package boundaries and the current runtime paths. They are development documentation, not published command reference pages.

## Runtime at a glance

`main.go` supplies build metadata, resolves the cluster base directory, creates the process container, and calls `cmd.ExecuteWithContext`. Command execution builds the canonical typed graph with [`di.NewApp`](di-container.md), adapts it with `di.NewAppContainer`, creates the deterministic built-in Cobra tree, and then attaches discovered external plugins for the production process. [`cmd/opencenter-local/main.go`](runtime-extensions-and-local-development.md) is a separate executable for local Kind, Gitea, and Flux workflows.

The main generation path is:

```text
cluster generate
  -> SetupService
  -> configuration load and validation
  -> CopyBase
  -> encrypted application render and promotion
  -> infrastructure render
  -> cluster Flux bridge
  -> non-Kind OpenTofu provisioning (optional in the path)
```

The renderer plans descriptor-owned and catalog-owned actions, validates output ownership, writes through atomic workspace operations, and promotes the result without deleting `custom/` content. Secret manifests are planned separately from encrypted overlay values.

## Map index

| Map | Scope | Start here when |
|---|---|---|
| [CLI commands](cli-commands.md) | Built-in Cobra registration and runtime plugin boundary | You need the command tree or registration path |
| [OpenStack provider and storage operations](openstack-provider-storage-operations.md) | Typed provider planning, one-service storage provisioning, credential effects, and persistence/recovery boundaries | You need OpenStack provider or storage operation behavior |
| [Cluster lifecycle](cluster-lifecycle.md) | Init, configure, validate, generate, deploy, destroy, import | You need an end-to-end operation flow |
| [Config system](config-system.md) | Loading, normalization, references, defaults, validation, persistence | You need config ownership or data shape |
| [DI container](di-container.md) | Typed app graph and compatibility container | You need constructor or service wiring |
| [GitOps engine](gitops-engine.md) | Generation entry points, templates, descriptors, atomic output | You need rendered repository behavior |
| [Providers](providers.md) | Provider config/deploy support, drift interfaces, bootstrap routing | You need provider capability boundaries |
| [Secrets management](secrets-management.md) | Secret manifests, SOPS overlays, key and ownership state | You need secret lifecycle behavior |
| [Rendering ownership and secret artifacts](rendering-ownership-and-secret-artifacts.md) | Detailed ownership planning and artifact materialization | You need to change renderer or secret output logic |
| [Runtime extensions and local development](runtime-extensions-and-local-development.md) | External plugins, `opencenter-local`, templates, security boundary | You need extension or local workflow behavior |
| [Import, operations, and resilience](import-operations-and-resilience.md) | Import scan/apply/report, drift, backup, locks, retries, circuits | You need operational recovery or import behavior |

## Directory ownership

| Directory | Responsibility | Related map |
|---|---|---|
| `cmd/` | Built-in Cobra commands, flags, output, and command orchestration | [CLI commands](cli-commands.md) |
| `cmd/opencenter-local/` | Separate local-development executable | [Runtime extensions and local development](runtime-extensions-and-local-development.md) |
| `internal/cluster/` | Cluster lifecycle services, provider bootstrap steps, and explicit provider/storage operations | [Cluster lifecycle](cluster-lifecycle.md), [OpenStack provider and storage operations](openstack-provider-storage-operations.md) |
| `internal/cluster/provider/openstack/` and `internal/cluster/storage/openstack/` | Typed provider planning, one-service storage provisioning, credential sequencing, and persistence/recovery | [OpenStack provider and storage operations](openstack-provider-storage-operations.md) |
| `internal/config/` and `internal/config/v2/` | CLI settings plus authoritative cluster configuration pipeline | [Config system](config-system.md) |
| `internal/core/paths/` and `internal/core/validation/` | Shared path resolution and validation primitives | [Config system](config-system.md) |
| `internal/di/` | Typed application graph and legacy container adapter | [DI container](di-container.md) |
| `internal/gitops/` | GitOps workspace generation, descriptors, catalogs, and validation | [GitOps engine](gitops-engine.md) |
| `internal/secretartifacts/` | Logical-secret to physical-artifact planning | [Rendering ownership and secret artifacts](rendering-ownership-and-secret-artifacts.md) |
| `internal/secrets/` and `internal/sops/` | Manifest synchronization, key state, SOPS encryption | [Secrets management](secrets-management.md) |
| `internal/cloud/` and `internal/credentials/` | Provider API/drift implementations and credential extraction | [Providers](providers.md) |
| `internal/cloud/openstack/profile.go`, `read_only_discovery.go`, and `storage.go` | Profile loading, read-only provider discovery, and storage provisioning adapters | [OpenStack provider and storage operations](openstack-provider-storage-operations.md) |
| `internal/importer/` | GitOps repository discovery, inference, patching, and reports | [Import, operations, and resilience](import-operations-and-resilience.md) |
| `internal/operations/` | Drift and backup interfaces/implementations | [Import, operations, and resilience](import-operations-and-resilience.md) |
| `internal/resilience/` | File/Redis locks, retry/backoff, and circuit breakers | [Import, operations, and resilience](import-operations-and-resilience.md) |
| `internal/localdev/` and `internal/plugins/` | Local infrastructure services and external CLI discovery | [Runtime extensions and local development](runtime-extensions-and-local-development.md) |
| `internal/security/` | Command construction, input validation, credential masking, audit logging | [Runtime extensions and local development](runtime-extensions-and-local-development.md) |
| `internal/template/` | Reusable Go template engine, caching, registries, and sandboxing | [GitOps engine](gitops-engine.md) |

## Cross-module boundaries

- Commands resolve lifecycle services from the typed app graph; they do not own rendering or provider implementation details.
- `internal/config/v2` produces the validated config consumed by lifecycle and GitOps code. Shared validation is in `internal/core/validation`.
- `internal/gitops` owns generated repository output. `internal/secretartifacts` plans secret targets without depending on a backend or renderer; `internal/secrets` materializes and tracks encrypted manifests.
- Provider config/deploy support, drift detection, bootstrap, provider planning, and storage provisioning are separate capabilities; see [Providers](providers.md) and [OpenStack provider and storage operations](openstack-provider-storage-operations.md). Provider apply and storage apply persist typed v2 configuration; storage apply additionally sequences remote actions and recovery.
- External plugins are runtime executables. The generated Cobra reference is built from `cmd.NewBuiltinRootCmd()` and does not load external plugins.

## Recommended reading order

For a command or workflow change, read [CLI commands](cli-commands.md), [DI container](di-container.md), and [Cluster lifecycle](cluster-lifecycle.md). For OpenStack provider or storage operations, continue with [OpenStack provider and storage operations](openstack-provider-storage-operations.md) and [Providers](providers.md). For generated output, continue with [GitOps engine](gitops-engine.md) and [Rendering ownership and secret artifacts](rendering-ownership-and-secret-artifacts.md). For operational behavior, read [Secrets management](secrets-management.md) and [Import, operations, and resilience](import-operations-and-resilience.md).
