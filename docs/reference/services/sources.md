---
id: service-sources
title: "Sources"
sidebar_label: Sources
description: Aggregate Flux GitRepository/OCIRepository sources consumed by every other rendered service.
doc_type: reference
audience: "platform engineers, contributors"
tags: [gitops, flux, sources, core, services]
---

> **Purpose:** For platform engineers and contributors, documents what the `sources` service represents and why nearly every other service's Kustomization depends on it.

## Overview

`sources` is not a deployable application; it represents the aggregate set of Flux `GitRepository`/`OCIRepository` source objects (emitted as `opencenter-sources` and the per-service `opencenter-<service>` sources under `services/sources/`) that every other service's generated `Kustomization` fetches its manifests from. It is a structural, foundational entry alongside [fluxcd](fluxcd.md) rather than a workload with its own Helm chart. It has no service-specific configuration beyond the shared `BaseConfig` fields (`internal/config/services/default_services.go` registers it as `DefaultServiceConfig`).

## Configuration

```yaml
opencenter:
  services:
    sources:
      enabled: true             # default: true
      namespace: flux-system     # default: flux-system
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `true` | Whether the aggregate sources are rendered |
| `namespace` | string | `flux-system` | Namespace for the Flux source objects |

## Dependencies

None enforced by `opencenter cluster service enable|disable`. Structurally, almost every other service's Flux `Kustomization` in the render catalog (`internal/gitops/render_catalog.go`) lists `sources` in its `OverrideDependsOn`, so Flux will not attempt to reconcile a service's manifests until the sources it fetches from exist.

## Rendering

`internal/gitops/auto_descriptor.go` special-cases `sources` (alongside [fluxcd](fluxcd.md)): it is always treated as explicitly owned and skipped by the generic auto-descriptor/render-catalog lookup used for other services, even though a `RenderSpec` entry for `sources` also exists in the built-in render catalog. The per-service source files themselves come from the descriptor-owned `services/sources/*.yaml.tpl` templates (see `internal/services/descriptors/data/services-sources-aggregate.yaml` and `managed-services-sources-aggregate.yaml` for the managed-service equivalent), aggregated into by each service's own descriptor via `aggregate_targets`.

## CLI commands

```bash
opencenter cluster service enable sources
opencenter cluster service disable sources
opencenter cluster service status
opencenter cluster service options sources
```
