---
id: service-kube-ovn
title: "Kube-OVN"
sidebar_label: Kube-OVN
description: Kube-OVN overlay CNI configuration fields and defaults, an alternative to Calico and Cilium.
doc_type: reference
audience: "platform engineers, operators"
tags: [networking, cni, kube-ovn, ovn, services]
---

> **Purpose:** For platform engineers and operators, documents the Kube-OVN service's configuration surface as an alternative CNI to Calico or Cilium.

## Overview

Kube-OVN is an OVN-based overlay CNI plugin, offered as an alternative to [Calico](calico.md) and [Cilium](cilium.md). It is **opt-in**: like Cilium, it has no entry in the default generated configuration (`internal/config/v2/defaults.go`), so it must be added explicitly.

## Configuration

```yaml
opencenter:
  services:
    kube-ovn:
      enabled: true
      namespace: ""                # no built-in default; set explicitly
      cilium_integration: false
      default_subnet: "10.16.0.0/16"
      version: ""
      enable_lb: false
      adoption_mode: managed
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | — (opt-in) | Whether Kube-OVN is deployed |
| `cilium_integration` | bool | `false` | Enable Cilium integration for load balancing (`KubeOVNConfig.CiliumIntegration`) |
| `default_subnet` | string | — | Default pod subnet, must be in CIDR notation (contain `/`) |
| `version` | string | — | Kube-OVN version to install; if set, must contain a `.` (semantic version) |
| `enable_lb` | bool | `false` | Enable load-balancing features |
| `namespace` | string | — | Namespace for Kube-OVN resources; no built-in default is applied by the CLI |
| `adoption_mode` | string | `managed` | See [Platform services architecture](../platform-services.md#adoption_mode) |

## Dependencies

None enforced by `opencenter cluster service enable|disable`. Kube-OVN, Calico, and Cilium are alternative CNI choices; the CLI does not itself prevent enabling more than one.

## Rendering

Kube-OVN has no dedicated YAML descriptor under `internal/services/descriptors/data/`. If enabled, it is rendered through the built-in render catalog (`internal/gitops/render_catalog.go`) using its `BaseConfig` fields, the same mechanism used for most other non-descriptor-backed services.

## CLI commands

```bash
opencenter cluster service enable kube-ovn
opencenter cluster service disable kube-ovn
opencenter cluster service status
opencenter cluster service options kube-ovn
```
