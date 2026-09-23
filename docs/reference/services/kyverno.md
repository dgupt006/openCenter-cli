---
id: service-kyverno
title: "Kyverno"
sidebar_label: Kyverno
description: Kubernetes-native policy engine configuration and defaults.
doc_type: reference
audience: "platform engineers, security engineers"
tags: [policy, security, admission-control, services]
---

> **Purpose:** For platform engineers and security engineers, documents the Kyverno service's configuration surface.

## Overview

Kyverno is a Kubernetes-native policy engine. It has no service-specific configuration beyond the shared `BaseConfig` fields (`internal/config/services/default_services.go` registers it as `DefaultServiceConfig`).

## Configuration

```yaml
opencenter:
  services:
    kyverno:
      enabled: true             # default: true
      namespace: kyverno         # default: kyverno
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `true` | Whether Kyverno is deployed |
| `namespace` | string | `kyverno` | Namespace for Kyverno resources |

## Dependencies

None enforced by `opencenter cluster service enable|disable`.

## Rendering

`kyverno` has no dedicated YAML descriptor; it is rendered through the built-in render catalog, which wires its Kustomization to depend on `sources` and `kyverno-base`.

## CLI commands

```bash
opencenter cluster service enable kyverno
opencenter cluster service disable kyverno
opencenter cluster service status
opencenter cluster service options kyverno
```
