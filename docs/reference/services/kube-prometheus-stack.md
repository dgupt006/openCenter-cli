---
id: service-kube-prometheus-stack
title: "kube-prometheus-stack"
sidebar_label: Prometheus Stack
description: Prometheus, Grafana, and Alertmanager configuration fields, storage sizing, and secrets.
doc_type: reference
audience: "platform engineers, operators"
tags: [prometheus, grafana, alertmanager, monitoring, observability, services]
---

> **Purpose:** For platform engineers and operators, documents kube-prometheus-stack's configuration surface, secrets, and rendering.

## Overview

`kube-prometheus-stack` deploys Prometheus, Grafana, and Alertmanager.

## Configuration

```yaml
opencenter:
  services:
    kube-prometheus-stack:
      enabled: true                          # default: true
      namespace: observability                # default: observability
      hostname:                               # deprecated alias for grafana_hostname
      grafana_hostname:
      prometheus_hostname:
      alertmanager_hostname:
      grafana_volume_size:
      grafana_storage_class:
      prometheus_volume_size:
      prometheus_storage_class:
      alertmanager_volume_size:
      alertmanager_storage_class:
      webhook_url:
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `true` | Whether the stack is deployed |
| `namespace` | string | `observability` | Namespace for all stack components |
| `hostname` | string | — | Deprecated alias for `grafana_hostname`; kept for backward compatibility |
| `grafana_hostname` | string | — | Grafana external hostname |
| `prometheus_hostname` | string | — | Prometheus external hostname |
| `alertmanager_hostname` | string | — | Alertmanager external hostname |
| `grafana_volume_size` | int | — | Grafana PVC size in GB |
| `grafana_storage_class` | string | — | Grafana storage class |
| `prometheus_volume_size` | int | — | Prometheus PVC size in GB |
| `prometheus_storage_class` | string | — | Prometheus storage class |
| `alertmanager_volume_size` | int | — | Alertmanager PVC size in GB |
| `alertmanager_storage_class` | string | — | Alertmanager storage class |
| `webhook_url` | string | — | Alertmanager webhook receiver URL |

### Validation

Volume sizes must be non-negative if set (`internal/services/plugins/prometheus_stack.go` — validation logic for this package, see [Platform services architecture](../platform-services.md) for why this validator is not currently the enforced path).

## Secrets

```yaml
secrets:
  grafana:
    admin_password:
    admin_user:
```

## Dependencies

None enforced by `opencenter cluster service enable|disable`. [alert-proxy](alert-proxy.md) and [rbac-manager](rbac-manager.md) render conditionally depending on whether `kube-prometheus-stack` is enabled, but this is a rendering-order relationship in the render catalog, not a validated CLI dependency.

## Rendering

`kube-prometheus-stack` has no dedicated YAML descriptor; it is rendered through the built-in render catalog. Its Flux Kustomization depends on `sources` and `envoy-gateway-api-base`. The generated `services/sources/kustomization.yaml.tpl` also includes an `opencenter-observability` source whenever `kube-prometheus-stack`, `loki`, `tempo`, `mimir`, or `opentelemetry-kube-stack` is enabled.

## CLI commands

```bash
opencenter cluster service enable kube-prometheus-stack
opencenter cluster service disable kube-prometheus-stack
opencenter cluster service status
opencenter cluster service options kube-prometheus-stack
```
