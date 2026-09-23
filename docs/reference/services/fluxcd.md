---
id: service-fluxcd
title: "FluxCD"
sidebar_label: FluxCD
description: Core GitOps reconciliation service configuration and structural rendering.
doc_type: reference
audience: "platform engineers, operators"
tags: [gitops, flux, continuous-delivery, core, services]
---

> **Purpose:** For platform engineers, documents the FluxCD core service and how it is treated structurally by the renderer.

## Overview

FluxCD is the GitOps reconciliation engine. It has no service-specific configuration beyond the shared `BaseConfig` fields (`internal/config/services/default_services.go` registers it as `DefaultServiceConfig`).

## Configuration

```yaml
opencenter:
  services:
    fluxcd:
      enabled: true             # default: true
      namespace: flux-system     # default: flux-system
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `true` | Whether FluxCD is deployed |
| `namespace` | string | `flux-system` | Namespace for Flux controllers |

## Dependencies

None enforced by `opencenter cluster service enable|disable`. Because almost every other service's generated Flux `Kustomization` depends on Flux itself being installed, disabling `fluxcd` in a generated cluster effectively breaks GitOps reconciliation cluster-wide, even though the CLI does not reject the change.

## Rendering

`fluxcd` is treated as structural: `internal/gitops/auto_descriptor.go` special-cases `fluxcd` (alongside [sources](sources.md)) so it is never routed through the generic auto-descriptor or render-catalog lookup used by other services, even though the built-in render catalog also carries a `RenderSpec` entry for it. Its Flux self-management manifests come from the root/aggregate descriptors (`internal/services/descriptors/data/root-overlay.yaml` and the `*-fluxcd-aggregate.yaml` descriptors).

## CLI commands

```bash
opencenter cluster service enable fluxcd
opencenter cluster service disable fluxcd
opencenter cluster service status
opencenter cluster service options fluxcd
```
