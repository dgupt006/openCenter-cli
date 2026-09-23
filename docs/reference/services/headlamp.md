---
id: service-headlamp
title: "Headlamp Dashboard"
sidebar_label: Headlamp
description: Kubernetes dashboard configuration, OIDC integration, secrets, and conditional dependency on Keycloak.
doc_type: reference
audience: "platform engineers, operators"
tags: [dashboard, ui, oidc, headlamp, services]
---

> **Purpose:** For platform engineers and operators, documents Headlamp's configuration surface and its conditional dependency on Keycloak.

## Overview

Headlamp provides a Kubernetes dashboard, optionally authenticated via OIDC.

## Configuration

```yaml
opencenter:
  services:
    headlamp:
      enabled: true                          # default: true
      namespace: headlamp                     # default: headlamp
      hostname:                                # default: dashboard.<cluster_fqdn>
      oidc_issuer_url:
      oidc_client_id:
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `true` | Whether Headlamp is deployed |
| `namespace` | string | `headlamp` | Namespace for Headlamp resources |
| `hostname` | string | `dashboard.<cluster_fqdn>` (set by the CLI on enable) | Public hostname for the dashboard |
| `oidc_issuer_url` | string | — | OIDC issuer URL |
| `oidc_client_id` | string | — | OIDC client ID |

## Secrets

```yaml
secrets:
  headlamp:
    oidc_client_secret:
```

## Dependencies

`internal/config/services/dependency_validator.go` enforces a conditional rule via `ValidateHeadlampOIDC`: **if `headlamp` is enabled and either `oidc_issuer_url` or `oidc_client_id` is set, `keycloak` must also be enabled.** If neither OIDC field is set, Headlamp has no enforced dependency.

## Rendering

`headlamp` has no dedicated YAML descriptor; it is rendered through the built-in render catalog using a dedicated Helm override-values template.

## CLI commands

```bash
opencenter cluster service enable headlamp
opencenter cluster service disable headlamp
opencenter cluster service status
opencenter cluster service options headlamp
```
