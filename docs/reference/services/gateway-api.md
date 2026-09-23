---
id: service-gateway-api
title: "Gateway API"
sidebar_label: Gateway API
description: Gateway API CRD installation configuration, consumed by the gateway service.
doc_type: reference
audience: "platform engineers, operators"
tags: [networking, gateway-api, crds, services]
---

> **Purpose:** For platform engineers and operators, documents the Gateway API CRD service that the [gateway](gateway.md) service depends on.

## Overview

`gateway-api` installs the Kubernetes Gateway API resources (Envoy Gateway's implementation) that the [gateway](gateway.md) service uses. It has no service-specific configuration beyond the shared `BaseConfig` fields (`internal/config/services/default_services.go` registers it as `DefaultServiceConfig`).

## Configuration

```yaml
opencenter:
  services:
    gateway-api:
      enabled: true                       # default: true
      namespace: envoy-gateway-system      # default: envoy-gateway-system
      adoption_mode: managed
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `true` | Whether Gateway API resources are deployed |
| `namespace` | string | `envoy-gateway-system` | Namespace for Envoy Gateway API resources |
| `adoption_mode` | string | `managed` | See [Platform services architecture](../platform-services.md#adoption_mode) |

## Dependencies

None enforced by `opencenter cluster service enable|disable`.

## Rendering

`gateway-api` has no dedicated YAML descriptor; it is rendered through the built-in render catalog (`internal/gitops/render_catalog.go`), which sets its Flux Kustomization name to `envoy-gateway-api` and applies override Helm values setting the Envoy Gateway logging level to `info`.

## CLI commands

```bash
opencenter cluster service enable gateway-api
opencenter cluster service disable gateway-api
opencenter cluster service status
opencenter cluster service options gateway-api
```
