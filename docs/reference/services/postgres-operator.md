---
id: service-postgres-operator
title: "PostgreSQL Operator"
sidebar_label: Postgres Operator
description: PostgreSQL operator configuration and its role as a keycloak dependency.
doc_type: reference
audience: "platform engineers, database administrators"
tags: [database, postgresql, operator, services]
---

> **Purpose:** For platform engineers, documents the postgres-operator service's configuration surface and why keycloak requires it.

## Overview

`postgres-operator` provisions and manages PostgreSQL clusters used by other services (e.g. [keycloak](keycloak.md)). It has no service-specific configuration beyond the shared `BaseConfig` fields (`internal/config/services/default_services.go` registers it as `DefaultServiceConfig`).

## Configuration

```yaml
opencenter:
  services:
    postgres-operator:
      enabled: true                    # default: true
      namespace: postgres-operator      # default: postgres-operator
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `true` | Whether the PostgreSQL operator is deployed |
| `namespace` | string | `postgres-operator` | Namespace for the operator |

## Dependencies

None of its own. [keycloak](keycloak.md) requires `postgres-operator` to be enabled — see `internal/config/services/dependency_validator.go`, enforced by `opencenter cluster service enable|disable`.

## Rendering

`postgres-operator` has no dedicated YAML descriptor; it is rendered through the built-in render catalog, with a fixed Helm override (`configGeneral.workers: 2`).

## CLI commands

```bash
opencenter cluster service enable postgres-operator
opencenter cluster service disable postgres-operator
opencenter cluster service status
opencenter cluster service options postgres-operator
```
