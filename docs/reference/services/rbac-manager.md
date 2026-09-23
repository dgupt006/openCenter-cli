---
id: service-rbac-manager
title: "RBAC Manager"
sidebar_label: RBAC Manager
description: Declarative RBAC management service configuration and defaults.
doc_type: reference
audience: "platform engineers, security engineers"
tags: [rbac, security, access-control, services]
---

> **Purpose:** For platform engineers, documents the RBAC Manager service's configuration surface.

## Overview

RBAC Manager provides declarative RBAC binding management via a CRD. It has no service-specific configuration beyond the shared `BaseConfig` fields (`internal/config/services/default_services.go` registers it as `DefaultServiceConfig`).

## Configuration

```yaml
opencenter:
  services:
    rbac-manager:
      enabled: true             # default: true
      namespace: rbac-system     # default: rbac-system
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `true` | Whether RBAC Manager is deployed |
| `namespace` | string | `rbac-system` | Namespace for RBAC Manager resources |

## Dependencies

None enforced by `opencenter cluster service enable|disable`.

## Rendering

`rbac-manager` has no dedicated YAML descriptor; it is rendered through the built-in render catalog as a base-only entry, with a conditional dependency on `kube-prometheus-stack-base` when `kube-prometheus-stack` is enabled.

## CLI commands

```bash
opencenter cluster service enable rbac-manager
opencenter cluster service disable rbac-manager
opencenter cluster service status
opencenter cluster service options rbac-manager
```
