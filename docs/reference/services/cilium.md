---
id: service-cilium
title: "Cilium"
sidebar_label: Cilium
description: Cilium eBPF-based CNI configuration fields and defaults, an alternative to Calico.
doc_type: reference
audience: "platform engineers, operators"
tags: [networking, cni, cilium, ebpf, services]
---

> **Purpose:** For platform engineers and operators, documents the Cilium service's configuration surface as an alternative CNI to Calico or Kube-OVN.

## Overview

Cilium is an eBPF-based CNI plugin, offered as an alternative to [Calico](calico.md) and [Kube-OVN](kube-ovn.md). Unlike Calico, Cilium is **opt-in**: it has no entry in the default generated configuration (`internal/config/v2/defaults.go`), so it must be added explicitly.

## Configuration

```yaml
opencenter:
  services:
    cilium:
      enabled: true
      namespace: ""                    # no built-in default; set explicitly
      operator_enabled: false
      kube_proxy_replacement: false
      module_source: ""
      adoption_mode: managed
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | — (opt-in) | Whether Cilium is deployed |
| `operator_enabled` | bool | `false` | Enable the Cilium operator for advanced features (`CiliumConfig.OperatorEnabled`) |
| `kube_proxy_replacement` | bool | `false` | Replace kube-proxy with Cilium's eBPF datapath |
| `module_source` | string | — | Cilium module source location |
| `namespace` | string | — | Namespace for Cilium resources; no built-in default is applied by the CLI |
| `adoption_mode` | string | `managed` | See [Platform services architecture](../platform-services.md#adoption_mode) |

## Dependencies

None enforced by `opencenter cluster service enable|disable`. Cilium, Calico, and Kube-OVN are alternative CNI choices; the CLI does not itself prevent enabling more than one.

## Rendering

Cilium has no dedicated YAML descriptor under `internal/services/descriptors/data/`. If enabled, it is rendered through the built-in render catalog (`internal/gitops/render_catalog.go`) using its `BaseConfig` fields (namespace, source, image), the same mechanism used for most other non-descriptor-backed services.

## CLI commands

```bash
opencenter cluster service enable cilium
opencenter cluster service disable cilium
opencenter cluster service status
opencenter cluster service options cilium
```
