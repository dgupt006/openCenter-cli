---
id: service-tempo
title: "Tempo"
sidebar_label: Tempo
description: Distributed tracing backend configuration, S3 storage backend, and unsupported Swift enum.
doc_type: reference
audience: "operators, platform engineers"
tags: [tempo, tracing, observability, s3, services]
---

> **Purpose:** For operators and platform engineers, documents Tempo's configuration fields, why `storage_type: swift` is rejected, and secrets.

## Overview

Tempo is the distributed tracing backend for openCenter clusters.

## Configuration

```yaml
opencenter:
  services:
    tempo:
      enabled: true                # default: true
      namespace: observability      # default: observability
      storage_type: s3               # default: s3
      bucket_name:
      volume_size:
      storage_class:
      s3_endpoint:
      s3_region:
      s3_credential_id:
      s3_force_path_style: false
      s3_insecure: false
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `true` | Whether Tempo is deployed |
| `namespace` | string | `observability` | Namespace for Tempo resources |
| `storage_type` | string | `s3` | Storage backend. `swift` is present in the schema enum for backward compatibility but is rejected — see below |
| `bucket_name` | string | — | Storage bucket/container name |
| `volume_size` | int | — | Persistent volume size in GB |
| `storage_class` | string | — | PVC storage class |
| `s3_endpoint` | string | — | S3 endpoint URL |
| `s3_region` | string | — | S3 region |
| `s3_credential_id` | string | — | OpenStack EC2 credential ID |
| `s3_force_path_style` | bool | `false` | Force S3 path-style addressing |
| `s3_insecure` | bool | `false` | Allow insecure (HTTP) S3 connections |

The schema and `TempoConfig` also retain a full set of `swift_*` fields (`swift_auth_url`, `swift_region`, `swift_auth_version`, `swift_application_credential_id`, `swift_container_name`, `swift_user_domain_name`, `swift_domain_name`) purely for backward compatibility; they are non-functional.

### Why Swift is rejected

Tempo's binary has no Swift storage backend upstream — it supports only S3/S3-compatible, GCS, and Azure Blob, and fails at startup with `unknown backend swift`. `internal/services/plugins/tempo.go`'s validator (dead-code path; see [Platform services architecture](../platform-services.md)) explicitly rejects `storage_type: swift` with a message pointing operators to use `s3` against a Swift S3-compatible endpoint instead. The live enable-time check in `cmd/cluster_service.go` requires a configured `s3_endpoint` plus matched access/secret keys for the resolved backend.

## Secrets

```yaml
secrets:
  tempo:
    access_key:
    secret_key:
    swift_application_credential_secret:   # retained for the deprecated swift_* fields only
```

## Dependencies

None enforced by `opencenter cluster service enable|disable`.

## Rendering

`tempo` has no dedicated YAML descriptor; it is rendered through the built-in render catalog with extra rendering-order dependencies on the observability namespace/sources and an override dependency on `sources`.

## CLI commands

```bash
opencenter cluster service enable tempo --secret="access_key=..." --secret="secret_key=..."
opencenter cluster service disable tempo
opencenter cluster service status
opencenter cluster service options tempo
```
