---
id: service-weave-gitops
title: "Weave GitOps"
sidebar_label: Weave GitOps
description: Weave GitOps dashboard configuration, secrets, and enforced dependency on FluxCD.
doc_type: reference
audience: "platform engineers, operators"
tags: [gitops, dashboard, flux, services]
---

> **Purpose:** For platform engineers, documents the Weave GitOps dashboard's configuration surface and its enforced dependency on FluxCD.

## Overview

Weave GitOps provides a web dashboard for FluxCD resources. It has no service-specific configuration beyond the shared `BaseConfig` fields (`internal/config/services/default_services.go` registers it as `DefaultServiceConfig`).

## Configuration

```yaml
opencenter:
  services:
    weave-gitops:
      enabled: false             # default: false
      namespace: flux-system      # default: flux-system
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `false` | Whether the Weave GitOps dashboard is deployed |
| `namespace` | string | `flux-system` | Namespace for the dashboard |

## Secrets

```yaml
secrets:
  weave_gitops:
    password:
    password_hash:
```

## Dependencies

`internal/config/services/dependency_validator.go` enforces: **`weave-gitops` requires `fluxcd` to be enabled.** This is checked by `opencenter cluster service enable|disable`.

## Rendering

`weave-gitops` has no dedicated YAML descriptor; it is rendered through the built-in render catalog, with an override Kustomization dependency on `sources` and `envoy-gateway-api-base`.

## CLI commands

```bash
opencenter cluster service enable weave-gitops --secret="password_hash=..."
opencenter cluster service disable weave-gitops
opencenter cluster service status
opencenter cluster service options weave-gitops
```
