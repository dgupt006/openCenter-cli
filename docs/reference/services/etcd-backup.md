---
id: service-etcd-backup
title: "etcd Backup"
sidebar_label: etcd Backup
description: Scheduled etcd snapshot backups to S3-compatible storage.
doc_type: reference
audience: "platform engineers, operators"
tags: [etcd, backup, disaster-recovery]
---

> **Purpose:** For platform engineers, documents the etcd backup service for scheduled cluster state snapshots to S3-compatible storage.

## Overview

The etcd backup service is disabled by default. When enabled, it runs a nightly CronJob at 01:00, saves an etcd snapshot, and uploads it to the configured S3-compatible bucket using SigV4. The service-specific SOPS-managed Secret is materialized into the generated kustomization. The backup image is pinned by digest.

## Configuration

When enabling the service, configure all required storage fields and service-specific credentials:

```yaml
opencenter:
  services:
    etcd-backup:
      enabled: true
      s3_endpoint: "https://s3.example.com"
      s3_bucket_name: "my-cluster-etcd-backups"
      s3_region: "us-east-1"

secrets:
  etcd_backup:
    access_key_id: "<access-key-id>"
    secret_access_key: "<secret-access-key>"
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `false` | Enable etcd snapshot backups |
| `s3_endpoint` | string | — | Required absolute HTTP(S) URL for the S3-compatible API. Runtime prefers this full endpoint. |
| `s3_bucket_name` | string | — | Required target bucket name; the configured value is used at runtime. |
| `s3_region` | string | — | Required S3 signing region. |
| `s3_host` | string | — | Legacy compatibility field. Keep it only for older configuration or lifecycle workflows; runtime prefers `s3_endpoint`. |
| `s3_credential_id` | string | — | Non-secret OpenStack credential lifecycle metadata; it is not stored as a pod secret. |

### Secrets

The following service-specific secrets are required when the service is enabled and are materialized through SOPS:

| Path | Description |
|------|-------------|
| `secrets.etcd_backup.access_key_id` | S3 access key ID |
| `secrets.etcd_backup.secret_access_key` | S3 secret access key |

Global AWS credentials are not used as a fallback for etcd-backup.

## Schedule and Runtime

- CronJob schedule: `0 1 * * *` (nightly at 01:00).
- S3 requests use SigV4.
- The configured `s3_bucket_name` is honored; the service does not substitute a built-in bucket name.
- The container image is pinned by digest.

## Dependencies

None.

## CLI Commands

```bash
opencenter cluster service enable etcd-backup
opencenter cluster service disable etcd-backup
opencenter cluster service status etcd-backup
opencenter cluster service options etcd-backup
```
