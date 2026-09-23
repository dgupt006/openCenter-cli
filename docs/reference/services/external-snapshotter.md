---
id: service-external-snapshotter
title: "External Snapshotter"
sidebar_label: External Snapshotter
description: CSI VolumeSnapshot CRDs and controller configuration and defaults.
doc_type: reference
audience: "platform engineers, storage administrators"
tags: [storage, csi, snapshots, services]
---

> **Purpose:** For platform engineers, documents the external snapshotter service's configuration surface.

## Overview

`external-snapshotter` installs the Kubernetes `VolumeSnapshot` CRDs and snapshot controller consumed by CSI drivers such as [openstack-csi](openstack-csi.md) and [vsphere-csi](vsphere-csi.md). It has no service-specific configuration beyond the shared `BaseConfig` fields (`internal/config/services/default_services.go` registers it as `DefaultServiceConfig`).

## Configuration

```yaml
opencenter:
  services:
    external-snapshotter:
      enabled: true                       # default: true
      namespace: external-snapshotter      # default: external-snapshotter
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `true` | Whether the snapshot CRDs and controller are deployed |
| `namespace` | string | `external-snapshotter` | Namespace for the snapshot controller |

## Dependencies

None enforced by `opencenter cluster service enable|disable`.

## Rendering

`external-snapshotter` has no dedicated YAML descriptor; it is rendered through the built-in render catalog as a base-only entry.

## CLI commands

```bash
opencenter cluster service enable external-snapshotter
opencenter cluster service disable external-snapshotter
opencenter cluster service status
opencenter cluster service options external-snapshotter
```
