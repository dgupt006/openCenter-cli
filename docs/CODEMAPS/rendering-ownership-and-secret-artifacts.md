---
id: rendering-ownership-and-secret-artifacts
title: "Explain Rendering Ownership and Secret Artifacts"
sidebar_label: Rendering Ownership
description: Explains how descriptors, the immutable render catalog, action validation, custom-file preservation, and secret artifact planning divide ownership during GitOps generation.
doc_type: explanation
audience: "contributors, maintainers"
tags: [rendering, ownership, descriptors, catalog, secrets]
---
# Rendering ownership and secret artifacts

The renderer separates three questions: which service is enabled, which code owns an output path, and which logical secret payload becomes a physical manifest. Keeping these decisions explicit prevents cross-service overwrites and unsafe adoption.

## Planning flow

```text
v2.Config
  -> descriptor registry + config view
  -> immutable RenderCatalog
  -> secretartifacts.Plan
  -> descriptor actions + dynamic catalog actions + auto-service actions
  -> validateClusterAppActions
  -> AtomicWriter workspace
  -> promotion to applications/overlays/<cluster>
```

`internal/gitops/descriptor_renderer.go` owns descriptor expansion, coverage checks, action planning, output normalization, and ownership validation. `internal/gitops/auto_descriptor.go` fills the gap for enabled services with no explicit descriptor, but only when a built-in catalog entry exists. `render_catalog.go` supplies direct function references and validates config ownership; it is not a mutable plugin registry.

## Ownership rules

| Decision | Source of truth | Consequence |
|---|---|---|
| Descriptor enabled | Service/managed-service config and descriptor conditions | Disabled or externally managed services produce no generated actions |
| Explicit file ownership | Descriptor roots/files | Every embedded template file must have one owner; duplicates/missing coverage fail planning |
| Dynamic service behavior | Immutable built-in `RenderCatalog` | Renderer behavior is compiled into the CLI and selected by service identity |
| Output safety | `validateClusterAppActions` and normalized paths | Actions cannot escape the target application workspace |
| User customization | `custom/` workspace subtree | Promotion preserves customer-owned custom content |
| Secret target | `secretartifacts.Artifact.TargetService` and `Path` | Physical manifest placement is independent of logical owner name |

## Secret artifact planning

`internal/secretartifacts/planner.go` is intentionally independent of secret backends and GitOps rendering. It:

- reads fixed secret blocks and `service_secrets` entries;
- normalizes service/key names and rejects unsafe service names;
- maps logical owners to a physical target and `secret.yaml` path;
- merges multiple owners deterministically, rejecting conflicting canonical keys;
- records all owners and source payloads; and
- validates that a materialized target is declared and enabled when topology is present.

The planner returns a sorted artifact list. The renderer may use artifact presence to decide whether a generated service should include a secret resource, while `internal/secrets` performs encrypted manifest synchronization and maintains ownership hashes.

## Write and promotion boundary

Application output is written in a temporary workspace with atomic file operations. The renderer can encrypt temporary override values before promotion. The promotion result reports added, updated, unchanged, seeded, and pruned generator-owned paths. Existing `custom/` content remains outside the generated ownership set.

This boundary is important for changes: a new renderer must first declare ownership, then produce a plan, then pass action containment and coverage validation. It must not write directly into the final overlay or infer ownership from whatever files happen to exist there.

## Related maps

- [GitOps engine](gitops-engine.md) — generation entry point and workspace lifecycle
- [Secrets management](secrets-management.md) — transactional encrypted manifest sync
- [Config system](config-system.md) — service and secret input model
