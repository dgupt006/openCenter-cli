---
id: service-openstack-csi
title: "OpenStack Cinder CSI"
sidebar_label: OpenStack CSI
description: OpenStack Cinder CSI driver configuration and defaults.
doc_type: reference
audience: "platform engineers, storage administrators"
tags: [openstack, storage, csi, services]
---

> **Purpose:** For platform engineers on OpenStack, documents the Cinder CSI driver's configuration surface.

## Overview

`openstack-csi` provides dynamic volume provisioning backed by OpenStack Cinder. It has no service-specific configuration beyond the shared `BaseConfig` fields (`internal/config/services/default_services.go` registers it as `DefaultServiceConfig`); it uses the cluster's OpenStack infrastructure credentials.

## Configuration

```yaml
opencenter:
  services:
    openstack-csi:
      enabled: true                 # default: true
      namespace: openstack-csi       # default: openstack-csi
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `true` | Whether the Cinder CSI driver is deployed |
| `namespace` | string | `openstack-csi` | Namespace for CSI driver resources |

## Dependencies

None enforced by `opencenter cluster service enable|disable`.

## Rendering

`openstack-csi` has no dedicated YAML descriptor; it is rendered through the built-in render catalog.

## CLI commands

```bash
opencenter cluster service enable openstack-csi
opencenter cluster service disable openstack-csi
opencenter cluster service status
opencenter cluster service options openstack-csi
```
