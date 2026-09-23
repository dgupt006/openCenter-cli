---
id: service-etcd-backup
title: "etcd Backup"
sidebar_label: etcd Backup
description: etcd snapshot backup service configuration, S3 endpoint fields, and secrets.
doc_type: reference
audience: "platform engineers, operators"
tags: [etcd, backup, disaster-recovery, services]
---

> **Purpose:** For platform engineers, documents the etcd backup service's configuration surface and required S3 credentials.

## Overview

The etcd backup service uploads etcd snapshots to an S3-compatible bucket. It is disabled by default.

## Configuration

```yaml
opencenter:
  services:
    etcd-backup:
      enabled: false               # default: false
      namespace: kube-system        # default: kube-system
      s3_host:
      s3_endpoint:
      s3_bucket_name:
      s3_credential_id:
      s3_region:
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `false` | Whether etcd snapshot backups are enabled |
| `namespace` | string | `kube-system` | Namespace for the backup CronJob |
| `s3_host` | string | — | S3-compatible endpoint host (legacy compatibility field) |
| `s3_endpoint` | string | — | S3-compatible endpoint URL |
| `s3_bucket_name` | string | — | S3 bucket name |
| `s3_credential_id` | string | — | OpenStack EC2 credential ID (non-secret lifecycle metadata, not a pod secret) |
| `s3_region` | string | — | S3 region |

## Secrets

```yaml
secrets:
  etcd_backup:
    access_key_id:
    secret_access_key:
```

Global AWS credentials are not used as a fallback for `etcd-backup`.

## Dependencies

None enforced by `opencenter cluster service enable|disable`.

## Rendering

`etcd-backup` has a dedicated descriptor (`internal/services/descriptors/data/service-etcd-backup.yaml`, `service: etcd-backup`) that owns everything under the `services/etcd-backup` template root and aggregates into `services-fluxcd-aggregate`.

## CLI commands

```bash
opencenter cluster service enable etcd-backup --param="s3_endpoint=https://s3.example.com" --param="s3_bucket_name=my-cluster-etcd-backups" --param="s3_region=us-east-1" --secret="access_key_id=..." --secret="secret_access_key=..."
opencenter cluster service disable etcd-backup
opencenter cluster service status
opencenter cluster service options etcd-backup
```
