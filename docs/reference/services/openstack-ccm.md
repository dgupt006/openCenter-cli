---
id: service-openstack-ccm
title: "OpenStack Cloud Controller Manager"
sidebar_label: OpenStack CCM
description: OpenStack Cloud Controller Manager configuration and defaults.
doc_type: reference
audience: "platform engineers, operators"
tags: [openstack, cloud-controller, networking, services]
---

> **Purpose:** For platform engineers on OpenStack, documents the Cloud Controller Manager service's configuration surface.

## Overview

`openstack-ccm` integrates Kubernetes with OpenStack infrastructure (load balancers, node metadata). It has no service-specific configuration beyond the shared `BaseConfig` fields (`internal/config/services/default_services.go` registers it as `DefaultServiceConfig`); it uses the cluster's OpenStack infrastructure credentials rather than a service-specific secret.

## Configuration

```yaml
opencenter:
  services:
    openstack-ccm:
      enabled: true                 # default: true
      namespace: openstack-ccm       # default: openstack-ccm
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `true` | Whether OpenStack CCM is deployed |
| `namespace` | string | `openstack-ccm` | Namespace for CCM resources |

## Dependencies

None enforced by `opencenter cluster service enable|disable`.

## Rendering

`openstack-ccm` has no dedicated YAML descriptor; it is rendered through the built-in render catalog.

## CLI commands

```bash
opencenter cluster service enable openstack-ccm
opencenter cluster service disable openstack-ccm
opencenter cluster service status
opencenter cluster service options openstack-ccm
```
