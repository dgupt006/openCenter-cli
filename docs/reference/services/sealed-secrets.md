---
id: service-sealed-secrets
title: "Sealed Secrets"
sidebar_label: Sealed Secrets
description: Encrypted Kubernetes Secret controller configuration and defaults.
doc_type: reference
audience: "platform engineers, operators"
tags: [secrets, encryption, gitops, services]
---

> **Purpose:** For platform engineers, documents the Sealed Secrets service's configuration surface.

## Overview

Sealed Secrets provides a cluster-side controller that decrypts `SealedSecret` resources into regular `Secret` objects, using asymmetric encryption. It has no service-specific configuration beyond the shared `BaseConfig` fields (`internal/config/services/default_services.go` registers it as `DefaultServiceConfig`).

## Configuration

```yaml
opencenter:
  services:
    sealed-secrets:
      enabled: true
      namespace: sealed-secrets     # default: sealed-secrets
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `false` | Whether Sealed Secrets is deployed |
| `namespace` | string | `sealed-secrets` | Namespace for the controller |

## Dependencies

None enforced by `opencenter cluster service enable|disable`.

## Rendering

`sealed-secrets` has no dedicated YAML descriptor; it is rendered through the built-in render catalog, with an override Kustomization dependency on `sources` and `sealed-secrets-namespace`.

## CLI commands

```bash
opencenter cluster service enable sealed-secrets
opencenter cluster service disable sealed-secrets
opencenter cluster service status
opencenter cluster service options sealed-secrets
```
