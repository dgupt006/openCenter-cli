---
id: service-opentelemetry-kube-stack
title: "OpenTelemetry Kube Stack"
sidebar_label: OpenTelemetry
description: OpenTelemetry collector configuration fields, exporters, and defaults.
doc_type: reference
audience: "platform engineers, operators"
tags: [observability, opentelemetry, tracing, services]
---

> **Purpose:** For platform engineers, documents the OpenTelemetry collector stack's configuration surface.

## Overview

`opentelemetry-kube-stack` deploys OpenTelemetry collectors.

## Configuration

```yaml
opencenter:
  services:
    opentelemetry-kube-stack:
      enabled: false                 # default: false
      namespace: observability        # default: observability
      collector_mode: deployment       # default: deployment; deployment | daemonset | statefulset
      collector_replicas: 1             # default: 1
      exporters:
        - name: tempo
          type: otlp                    # otlp | prometheus | jaeger
          endpoint: tempo.observability.svc:4317
          headers: {}
      processors:
        - batch
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `false` | Whether the collector stack is deployed |
| `namespace` | string | `observability` | Namespace for collector resources |
| `collector_mode` | string | `deployment` | `deployment` \| `daemonset` \| `statefulset` |
| `collector_replicas` | int | `1` | Number of collector replicas |
| `exporters` | list of `OTelExporter` | — | Export destinations |
| `exporters[].name` | string | required | Exporter identifier |
| `exporters[].type` | string | required | `otlp` \| `prometheus` \| `jaeger` |
| `exporters[].endpoint` | string | required | Destination endpoint URL |
| `exporters[].headers` | map of strings | — | Additional HTTP headers |
| `processors` | list of strings | — | Processor pipeline stage names |

## Dependencies

None enforced by `opencenter cluster service enable|disable`.

## Rendering

`opentelemetry-kube-stack` has no dedicated YAML descriptor; if enabled, it is rendered through the built-in render catalog. Enabling it also causes the generated `services/sources/kustomization.yaml.tpl` to include the shared `opencenter-observability` source.

## CLI commands

```bash
opencenter cluster service enable opentelemetry-kube-stack
opencenter cluster service disable opentelemetry-kube-stack
opencenter cluster service status
opencenter cluster service options opentelemetry-kube-stack
```
