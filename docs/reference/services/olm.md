---
id: service-olm
title: "Operator Lifecycle Manager"
sidebar_label: OLM
description: Operator Lifecycle Manager configuration and its role as a keycloak dependency.
doc_type: reference
audience: "platform engineers, operators"
tags: [operators, lifecycle, olm, services]
---

> **Purpose:** For platform engineers, documents the OLM service's configuration surface and why keycloak requires it.

## Overview

OLM (Operator Lifecycle Manager) installs and manages Kubernetes operators from catalog sources. It has no service-specific configuration beyond the shared `BaseConfig` fields (`internal/config/services/default_services.go` registers it as `DefaultServiceConfig`).

## Configuration

```yaml
opencenter:
  services:
    olm:
      enabled: true       # default: true
      namespace: olm        # default: olm
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `true` | Whether OLM is deployed |
| `namespace` | string | `olm` | Namespace for OLM resources |

## Dependencies

None of its own. [keycloak](keycloak.md) requires `olm` to be enabled — see `internal/config/services/dependency_validator.go`, enforced by `opencenter cluster service enable|disable`.

## Rendering

`olm` has a dedicated descriptor (`internal/services/descriptors/data/service-olm.yaml`, `service: olm`) covering its sources, Kustomization, and a bundle-unpack `NetworkPolicy`, and aggregates into `services-fluxcd-aggregate` and `services-sources-aggregate`.

## CLI commands

```bash
opencenter cluster service enable olm
opencenter cluster service disable olm
opencenter cluster service status
opencenter cluster service options olm
```
