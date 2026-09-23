---
id: service-harbor
title: "Harbor"
sidebar_label: Harbor
description: Container registry configuration, storage sizing, database options, and required S3 secrets.
doc_type: reference
audience: "platform engineers, operators"
tags: [registry, containers, harbor, services]
---

> **Purpose:** For platform engineers, documents Harbor's configuration surface, storage/database options, and its required S3 credentials.

## Overview

Harbor is a container registry, deployed with S3-backed image storage.

## Configuration

```yaml
opencenter:
  services:
    harbor:
      enabled: false                    # default: false
      namespace: harbor                  # default: harbor
      hostname:
      external_url:
      storage_type: s3                    # default: s3 (only s3 is a valid value)
      registry_volume_size: 100            # default: 100 (min 1)
      jobservice_volume_size: 10            # default: 10 (min 1; min 10 on Cinder-backed regions e.g. Rackspace SJC3)
      database_volume_size: 10              # default: 10 (min 1)
      redis_volume_size: 10                 # default: 10 (min 1; min 10 on Cinder-backed regions)
      trivy_volume_size: 10                 # default: 10 (min 1; min 10 on Cinder-backed regions)
      storage_class:
      s3_bucket:
      s3_region:
      s3_endpoint:
      database_type: internal               # default: internal; internal | external
      database_host:
      database_port:
      database_name:
      database_user:
      emit_certificate: false
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `false` | Whether Harbor is deployed |
| `namespace` | string | `harbor` | Namespace for Harbor resources |
| `hostname` | string | — | Harbor external hostname |
| `external_url` | string | — | External URL for Harbor; must be `http://` or `https://` when set |
| `storage_type` | string | `s3` | Only `s3` is a valid value |
| `registry_volume_size` | int | `100` | Registry PVC size in GB (min 1); retained for compatibility and required Harbor cache/state |
| `jobservice_volume_size` | int | `10` | Jobservice log PVC size in GB (min 1; min 10 on Cinder-backed regions e.g. Rackspace SJC3) |
| `database_volume_size` | int | `10` | Internal database PVC size in GB (min 1) |
| `redis_volume_size` | int | `10` | Redis PVC size in GB (min 1; min 10 on Cinder-backed regions) |
| `trivy_volume_size` | int | `10` | Trivy PVC size in GB (min 1; min 10 on Cinder-backed regions) |
| `storage_class` | string | infrastructure `storage.default_storage_class` | Storage class for Harbor PVCs |
| `s3_bucket` | string | — | S3 bucket name for image storage |
| `s3_region` | string | — | S3 region |
| `s3_endpoint` | string | — | S3-compatible endpoint URL; must be a valid URL when set |
| `database_type` | string | `internal` | `internal` \| `external` |
| `database_host` / `database_port` / `database_name` / `database_user` | — | — | Required when `database_type: external` |
| `emit_certificate` | bool | — | Render the Harbor TLS certificate manifest via cert-manager |

The five volume-size defaults above (`100`/`10`/`10`/`10`/`10`) are applied by `HarborConfig.UnmarshalYAML` only when the corresponding field is omitted from YAML — an explicitly configured `0` is preserved so runtime validation can reject it.

## Secrets

```yaml
secrets:
  harbor:
    admin_password:
    database_password:
    registry_password:
    s3_access_key_id:
    s3_secret_access_key:
```

### Validation

`opencenter cluster service enable harbor` requires the Harbor S3 access key and secret key to be provided together (both set, or both absent).

## Dependencies

None enforced by `opencenter cluster service enable|disable`.

## Rendering

Harbor has a dedicated descriptor (`internal/services/descriptors/data/service-harbor.yaml`, `service: harbor`) covering its `HTTPRoute`, Kustomization, Helm override values, and — conditionally, when `emit_certificate: true` — its `Certificate` manifest. It aggregates into `services-fluxcd-aggregate` and `services-sources-aggregate`.

## CLI commands

```bash
opencenter cluster service enable harbor --secret="s3_access_key_id=..." --secret="s3_secret_access_key=..."
opencenter cluster service disable harbor
opencenter cluster service status
opencenter cluster service options harbor
```
