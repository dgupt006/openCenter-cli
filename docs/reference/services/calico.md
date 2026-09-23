---
id: service-calico
title: "Calico"
sidebar_label: Calico
description: Calico CNI configuration fields, defaults, and rendering for pod networking and network policy.
doc_type: reference
audience: "platform engineers, operators"
tags: [networking, cni, calico, services]
---

> **Purpose:** For platform engineers and operators, documents the Calico service's configuration surface, default state, and how it is rendered.

## Overview

Calico is a CNI plugin that provides pod-to-pod networking and Kubernetes `NetworkPolicy` enforcement. It is the default CNI in generated openCenter cluster configuration.

## Configuration

```yaml
opencenter:
  services:
    calico:
      enabled: true                # default: true
      namespace: calico-system     # default: calico-system
      kube_api_server: ""          # optional
      adoption_mode: managed       # managed | external | sync | deferred | takeover
      source:
        repo: ""
        branch: ""
        release: ""
      image:
        repository: ""
        tag: ""
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `true` | Whether Calico is deployed (`internal/config/services/calico.go`) |
| `namespace` | string | `calico-system` | Namespace for Calico resources |
| `kube_api_server` | string | — | Calico Kubernetes API server address (`CalicoConfig.KubeAPIServer`) |
| `adoption_mode` | string | `managed` | See [Platform services architecture](../platform-services.md#adoption_mode) |
| `source.repo` / `source.branch` / `source.release` | string | — | GitOps source override |
| `image.repository` / `image.tag` | string | — | Container image override |

## Dependencies

None enforced by `opencenter cluster service enable|disable`.

## Rendering

Calico has a dedicated descriptor (`internal/services/descriptors/data/service-calico.yaml`, `service: calico`) that owns every file under the `services/calico` template root; it has no conditional (`when`) files and does not aggregate into the shared Flux/sources kustomizations.

## CLI commands

```bash
opencenter cluster service enable calico
opencenter cluster service disable calico
opencenter cluster service status
opencenter cluster service options calico
```
