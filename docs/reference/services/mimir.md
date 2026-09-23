---
id: service-mimir
title: "Grafana Mimir"
sidebar_label: Mimir
description: Long-term metrics storage service configuration and defaults.
doc_type: reference
audience: "platform engineers, operators"
tags: [monitoring, metrics, mimir, services]
---

> **Purpose:** For platform engineers and operators, documents the Mimir service's configuration surface.

## Overview

Mimir provides long-term metrics storage. It has no service-specific configuration beyond the shared `BaseConfig` fields (`internal/config/services/default_services.go` registers it as `DefaultServiceConfig`).

## Configuration

```yaml
opencenter:
  services:
    mimir:
      enabled: false               # default: false
      namespace: observability      # default: observability
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `false` | Whether Mimir is deployed |
| `namespace` | string | `observability` | Namespace for Mimir resources |

## Secrets

`schema/opencenter-v2.schema.json` defines `secrets.mimir.swift_application_credential_secret`; global AWS application credentials (`secrets.global.aws.application.*`) are also present in the schema for S3-backed setups.

## Dependencies

None enforced by `opencenter cluster service enable|disable`.

## Rendering

`mimir` has no dedicated YAML descriptor; it is rendered through the built-in render catalog with extra rendering-order dependencies on the observability namespace/sources and an override dependency on `sources`. Enabling `mimir` also causes the generated `services/sources/kustomization.yaml.tpl` to include the shared `opencenter-observability` source.

## CLI commands

```bash
opencenter cluster service enable mimir
opencenter cluster service disable mimir
opencenter cluster service status
opencenter cluster service options mimir
```
