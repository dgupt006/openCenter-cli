---
id: config-system-map
title: "Explain the Configuration System"
sidebar_label: Config System
description: Explains authoritative configuration loading, normalization, reference resolution, default hydration, validation, path resolution, and atomic persistence.
doc_type: explanation
audience: "contributors, maintainers, operators"
tags: [configuration, yaml, validation, defaults, paths]
---
# Configuration system

The cluster configuration boundary is the v2 model and loader. The top-level `internal/config` package retains CLI settings and compatibility helpers; `internal/config/v2` owns the cluster configuration lifecycle.

## Load flow

```text
cluster identifier
  -> internal/core/paths.PathResolver
  -> internal/config/v2 ConfigurationManager / ConfigIOHandler
  -> ConfigLoader
       1. Decode public YAML
       2. Normalize aliases and canonical forms
       3. Resolve ${ref:}, ${env:}, and ${file:} references
       4. Apply provider/region defaults
       5. Validate schema, business, provider, deployment, and service rules
       6. Freeze the returned model for use
  -> validated v2.Config
```

`ConfigurationManager` checks its cache before resolving a path and loading from disk. Saves validate, serialize through the public config representation, and use atomic filesystem writes.

## Package ownership

| Package | Owns |
|---|---|
| `internal/config/` | CLI settings, config/state directory compatibility helpers, status updates, and constructors used by commands |
| `internal/config/v2/` | `Config` types, `ConfigLoader`, `ConfigurationManager`, public encode/decode, reference resolver, defaults integration, and validation orchestration |
| `internal/config/defaults/` | Provider/region default registry and hydration without overwriting explicit values |
| `internal/config/flags/` | `cluster set` and init flag parsing, path mutation, merges, masking, and dry-run formatting |
| `internal/config/overlay/` | Overlay unit and customer-managed configuration types |
| `internal/config/cache/` | Named cache utilities used by configuration support code |
| `internal/config/persistence/` | YAML/path persistence helpers used by compatibility code |
| `internal/config/registry/` | Service configuration type registration |
| `internal/config/services/` | Typed service configs, adoption modes, provider compatibility, dependencies, and secret requirements |
| `internal/config/v2schema/` | JSON Schema generation and schema checks for editor support |
| `internal/core/paths/` | Organization-aware identifiers, secure path resolution, caches, and `ClusterPaths` |
| `internal/core/validation/` | Shared validation engine and validator implementations |

Shared validation belongs to `internal/core/validation`.

## Data and persistence boundaries

- YAML on disk is decoded through the public v2 decoder rather than a permissive internal-only shape.
- Reference resolution occurs before provider-region hydration and validation.
- `ConfigManager`/`ConfigurationManager` caches are an optimization, not a second source of truth.
- Atomic saves protect cluster config files; generated GitOps files have their own workspace/promotion transaction described in [GitOps engine](gitops-engine.md).
- `PathResolver` prevents identifier/path ambiguity and keeps organization-aware layouts consistent across commands, local development, import, and secrets.

## Consumers

| Consumer | Uses configuration for |
|---|---|
| `internal/cluster` | Init, guided configuration, readiness validation, generation, bootstrap, and destroy |
| `internal/gitops` | Descriptor conditions, render catalog decisions, infrastructure templates, Flux bridge, and overlay values |
| `internal/secretartifacts` | Logical secret owners, target services, and physical artifact paths |
| `internal/secrets` | Manifest payloads, overlay paths, SOPS key references, and ownership state |
| `internal/importer` | Proposed configs and high-confidence YAML patches |
| `internal/localdev` | Resolving a cluster before local Gitea/Flux operations |

## Related maps

- [Cluster lifecycle](cluster-lifecycle.md) — config as lifecycle input
- [Rendering ownership and secret artifacts](rendering-ownership-and-secret-artifacts.md) — config ownership at render time
- [Secrets management](secrets-management.md) — config-to-secret synchronization
- [DI container](di-container.md) — construction of config managers and services
