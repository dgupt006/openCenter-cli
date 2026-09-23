---
id: service-longhorn
title: "Longhorn"
sidebar_label: Longhorn
description: Longhorn distributed block storage configuration, replica settings, and backup targets.
doc_type: reference
audience: "platform engineers, storage administrators"
tags: [storage, block-storage, distributed, services]
---

> **Purpose:** For platform engineers, documents Longhorn's configuration surface: replica settings, provisioning thresholds, and backup targets.

## Overview

Longhorn provides distributed block storage for Kubernetes, replicating volume data across nodes.

## Configuration

```yaml
opencenter:
  services:
    longhorn:
      enabled: false                                    # default: false
      namespace: longhorn-system                          # default: longhorn-system
      hostname:                                            # default: longhorn.<cluster_fqdn>
      default_replica_count: 3                             # default: 3
      default_data_path: /var/lib/longhorn                 # default: /var/lib/longhorn
      storage_over_provisioning_percentage: 200             # default: 200
      storage_minimal_available_percentage: 25              # default: 25
      backup_target:                                        # s3:// or nfs://
      backup_target_credential_secret:
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `false` | Whether Longhorn is deployed |
| `namespace` | string | `longhorn-system` | Namespace for Longhorn resources |
| `hostname` | string | `longhorn.<cluster_fqdn>` (set by the CLI on enable) | UI ingress hostname |
| `default_replica_count` | int | `3` | Number of replicas per volume |
| `default_data_path` | string | `/var/lib/longhorn` | Node storage path for volume data |
| `storage_over_provisioning_percentage` | int | `200` | Allowed over-provisioning percentage |
| `storage_minimal_available_percentage` | int | `25` | Minimum available storage before scheduling stops |
| `backup_target` | string | — | Backup destination (`s3://` or `nfs://`) |
| `backup_target_credential_secret` | string | — | Secret name holding backup target credentials |

## Dependencies

None enforced by `opencenter cluster service enable|disable`.

## Rendering

`longhorn` has no dedicated YAML descriptor; it is rendered through the built-in render catalog, with an override Kustomization dependency on `sources`, `longhorn-base`, and `envoy-gateway-api-base`.

## CLI commands

```bash
opencenter cluster service enable longhorn
opencenter cluster service disable longhorn
opencenter cluster service status
opencenter cluster service options longhorn
```
