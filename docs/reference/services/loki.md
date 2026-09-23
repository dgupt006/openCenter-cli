---
id: service-loki
title: "Loki"
sidebar_label: Loki
description: Log aggregation service configuration, S3 and Swift storage backends, and secret fallback.
doc_type: reference
audience: "operators, platform engineers"
tags: [loki, logging, observability, s3, swift, services]
---

> **Purpose:** For operators and platform engineers, documents Loki's configuration fields, storage backends, secrets, and validation.

## Overview

Loki is the log aggregation backend for openCenter clusters, with S3 or Swift object storage.

## Configuration

```yaml
opencenter:
  services:
    loki:
      enabled: true                  # default: true
      namespace: observability        # default: observability
      storage_type: swift              # default: swift; s3 | swift
      bucket_name:
      volume_size:
      storage_class:

      # Swift backend — authenticates with a Keystone username/password,
      # NOT application credentials.
      swift_auth_url:                  # must end in /v3
      swift_region:
      swift_auth_version: 3             # default: 3
      swift_username:
      swift_project_name:
      swift_project_domain_name:        # defaults to swift_domain_name
      swift_container_name:
      swift_user_domain_name:
      swift_domain_name:
      swift_application_credential_id:  # deprecated: not honored by Loki's Swift driver

      # S3 backend
      s3_endpoint:
      s3_region:
      s3_credential_id:
      s3_force_path_style: false
      s3_insecure: false
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `true` | Whether Loki is deployed |
| `namespace` | string | `observability` | Namespace for Loki resources |
| `storage_type` | string | `swift` | `s3` or `swift` |
| `bucket_name` | string | — | Storage bucket/container name |
| `volume_size` | int | — | Persistent volume size in GB |
| `storage_class` | string | — | PVC storage class |
| `swift_auth_url` | string | — | Swift Keystone V3 auth URL (must end `/v3`) |
| `swift_region` | string | — | Swift region name |
| `swift_auth_version` | int | `3` | Swift auth version |
| `swift_username` | string | — | Swift Keystone username for username/password auth (this is what Loki's driver actually honors) |
| `swift_project_name` | string | — | Swift Keystone project name |
| `swift_project_domain_name` | string | — | Defaults to `swift_domain_name` |
| `swift_container_name` | string | — | Swift container for Loki logs |
| `swift_user_domain_name` | string | — | Swift user domain name |
| `swift_domain_name` | string | — | Swift domain name |
| `swift_application_credential_id` | string | — | Deprecated: not honored by Loki's Swift driver; use `swift_username` + the `swift_password` secret instead |
| `s3_endpoint` | string | — | S3 endpoint URL |
| `s3_region` | string | — | S3 region |
| `s3_credential_id` | string | — | OpenStack EC2 credential ID |
| `s3_force_path_style` | bool | `false` | Force S3 path-style addressing |
| `s3_insecure` | bool | `false` | Allow insecure (HTTP) S3 connections |

### Validation

`internal/services/plugins/loki.go` (dead-code validator; see [Platform services architecture](../platform-services.md)) requires `swift_auth_url` when `storage_type: swift` and `s3_endpoint` when `storage_type: s3`. The live path in `cmd/cluster_service.go` requires, for the resolved backend: a configured `s3_endpoint` plus matched S3 access/secret keys for `s3`, or a matched Swift application-credential ID/secret for `swift`.

## Secrets

```yaml
secrets:
  loki:
    s3_access_key_id:
    s3_secret_access_key:
    swift_application_credential_secret:
    swift_password:
```

## Dependencies

None enforced by `opencenter cluster service enable|disable`.

## Rendering

`loki` has no dedicated YAML descriptor; it is rendered through the built-in render catalog with extra rendering-order dependencies on the observability namespace/sources and an override dependency on `sources`.

## CLI commands

```bash
opencenter cluster service enable loki --param="storage_type=s3" --secret="s3_access_key_id=..." --secret="s3_secret_access_key=..."
opencenter cluster service disable loki
opencenter cluster service status
opencenter cluster service options loki
```
