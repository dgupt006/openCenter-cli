---
id: service-velero
title: "Velero"
sidebar_label: Velero
description: Cluster backup and disaster recovery configuration, storage backends, and secrets.
doc_type: reference
audience: "operators, platform engineers"
tags: [velero, backup, disaster-recovery, services]
---

> **Purpose:** For operators and platform engineers, documents Velero's configuration surface, storage backends, and secrets.

## Overview

Velero provides backup and disaster recovery for Kubernetes cluster resources and persistent volumes.

## Configuration

```yaml
opencenter:
  services:
    velero:
      enabled: true              # default: true
      namespace: velero           # default: velero
      backup_bucket:
      region:
      s3_endpoint:
      s3_region:
      s3_credential_id:
      s3_force_path_style: false
      s3_insecure: false
      storage_type: s3             # default: s3; s3 | swift | gcs | azure
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `true` | Whether Velero is deployed |
| `namespace` | string | `velero` | Namespace for Velero resources |
| `backup_bucket` | string | — | Backup bucket name; required by the plugin validator when enabled |
| `region` | string | — | Backup region |
| `s3_endpoint` | string | — | S3-compatible endpoint URL |
| `s3_region` | string | — | S3 region |
| `s3_credential_id` | string | — | OpenStack EC2 credential ID |
| `s3_force_path_style` | bool | `false` | Force S3 path-style addressing |
| `s3_insecure` | bool | `false` | Allow insecure (HTTP) connections |
| `storage_type` | string | `s3` | `s3` \| `swift` \| `gcs` \| `azure` |

### Validation

`internal/services/plugins/velero.go` (dead-code validator; see [Platform services architecture](../platform-services.md)) requires `backup_bucket` when `enabled: true`.

## Secrets

The `internal/config/services/provider_registry.go` compatibility matrix picks a default `storage_type` from the infrastructure provider (`s3` for AWS/bare-metal/vSphere, `swift` for OpenStack, `gcs` for GCP, `azure` for Azure). `schema/opencenter-v2.schema.json` defines:

```yaml
secrets:
  velero:
    access_key_id:
    secret_access_key:
```

## Dependencies

None enforced by `opencenter cluster service enable|disable`.

## Rendering

`velero` has no dedicated YAML descriptor; it is rendered through the built-in render catalog with an extra rendering-order dependency on its own override values and an override Kustomization dependency on `sources` and `velero-namespace`.

## CLI commands

```bash
opencenter cluster service enable velero --param="backup_bucket=my-cluster-backups"
opencenter cluster service disable velero
opencenter cluster service status
opencenter cluster service options velero
```
