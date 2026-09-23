---
id: service-vsphere-csi
title: "vSphere CSI Driver"
sidebar_label: vSphere CSI
description: VMware vSphere CSI driver configuration, storage classes, secrets, and defaults.
doc_type: reference
audience: "platform engineers, operators"
tags: [storage, vsphere, vmware, csi, services]
---

> **Purpose:** For platform engineers and operators, documents the vSphere CSI driver's configuration surface for VMware environments.

## Overview

`vsphere-csi` provides dynamic volume provisioning backed by VMware vSphere datastores.

## Configuration

```yaml
opencenter:
  services:
    vsphere-csi:
      enabled: false                        # default: false
      namespace: vmware-system-csi           # default: vmware-system-csi
      storage_classes:
        - name: vsphere-default
          datastore_url: "ds:///vmfs/volumes/datastore1/"
          reclaim_policy: Retain              # Retain | Delete, default: Retain
          volume_binding_mode: Immediate       # Immediate | WaitForFirstConsumer, default: Immediate
          allow_expansion: true                # default: true
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `false` | Whether the vSphere CSI driver is deployed |
| `namespace` | string | `vmware-system-csi` | Namespace for CSI resources |
| `storage_classes` | list of `VSphereStorageClass` | — | Storage class definitions |
| `storage_classes[].name` | string | required | StorageClass name |
| `storage_classes[].datastore_url` | string | required | vSphere datastore URL |
| `storage_classes[].reclaim_policy` | string | `Retain` | `Retain` or `Delete` |
| `storage_classes[].volume_binding_mode` | string | `Immediate` | `Immediate` or `WaitForFirstConsumer` |
| `storage_classes[].allow_expansion` | bool | `true` | Allow online volume expansion |

## Secrets

`schema/opencenter-v2.schema.json` defines `secrets.vsphere_csi`:

```yaml
secrets:
  vsphere_csi:
    vcenter_host:
    username:
    password:
    datacenters:
    insecure_flag:
    port:
    datastoreurl:
```

## Dependencies

None enforced by `opencenter cluster service enable|disable`.

## Rendering

`vsphere-csi` has no dedicated YAML descriptor; if enabled, it is rendered through the built-in render catalog.

## CLI commands

```bash
opencenter cluster service enable vsphere-csi
opencenter cluster service disable vsphere-csi
opencenter cluster service status
opencenter cluster service options vsphere-csi
```
