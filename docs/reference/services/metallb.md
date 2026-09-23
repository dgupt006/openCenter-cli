---
id: service-metallb
title: "MetalLB"
sidebar_label: MetalLB
description: MetalLB IP address pool and L2 advertisement configuration for bare-metal LoadBalancer services.
doc_type: reference
audience: "platform engineers, operators"
tags: [networking, load-balancer, bare-metal, services]
---

> **Purpose:** For platform engineers, documents MetalLB configuration: IP address pools and L2 advertisements.

## Overview

MetalLB provides a `LoadBalancer`-type Service implementation for clusters that are not on a cloud provider with a built-in load balancer. It is disabled by default.

## Configuration

```yaml
opencenter:
  services:
    metallb:
      enabled: true
      namespace: metallb-system     # default: metallb-system
      ip_address_pools:
        - name: public-pool
          addresses:
            - 72.4.119.48/28
          auto_assign: true
          avoid_buggy_ips: false
      l2_advertisements:
        - name: public-pool-l2
          ip_address_pools:
            - public-pool
          interfaces:
            - eth0
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `false` | Whether MetalLB is deployed |
| `namespace` | string | `metallb-system` | Namespace for generated MetalLB resources |
| `ip_address_pools` | list | — | `IPAddressPool` definitions |
| `ip_address_pools[].name` | string | required | Pool identifier |
| `ip_address_pools[].addresses` | list of strings | required | IP ranges (CIDR or `start-end`) |
| `ip_address_pools[].auto_assign` | bool | `true` when omitted (`GetAutoAssign()`) | Automatically assign IPs from this pool |
| `ip_address_pools[].avoid_buggy_ips` | bool | `false` | Avoid `.0`/`.255` addresses |
| `l2_advertisements` | list | — | `L2Advertisement` definitions |
| `l2_advertisements[].name` | string | required | Advertisement identifier |
| `l2_advertisements[].ip_address_pools` | list of strings | all pools if empty | Pools this advertisement selects |
| `l2_advertisements[].interfaces` | list of strings | — | Node interfaces to advertise on |

If `l2_advertisements` is omitted entirely, no L2 advertisement is generated.

## Dependencies

None enforced by `opencenter cluster service enable|disable`.

## Rendering

`metallb` has no dedicated YAML descriptor; it is rendered through the built-in render catalog.

## CLI commands

```bash
opencenter cluster service enable metallb
opencenter cluster service disable metallb
opencenter cluster service status
opencenter cluster service options metallb
```
