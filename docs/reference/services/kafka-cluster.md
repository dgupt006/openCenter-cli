---
id: service-kafka-cluster
title: "Kafka Cluster"
sidebar_label: Kafka Cluster
description: Apache Kafka (Strimzi) service configuration and defaults.
doc_type: reference
audience: "platform engineers, operators"
tags: [kafka, streaming, strimzi, services]
---

> **Purpose:** For platform engineers, documents the Kafka cluster service's configuration surface.

## Overview

`kafka-cluster` deploys Apache Kafka via the Strimzi operator. It has no service-specific configuration beyond the shared `BaseConfig` fields (`internal/config/services/default_services.go` registers it as `DefaultServiceConfig`).

## Configuration

```yaml
opencenter:
  services:
    kafka-cluster:
      enabled: false               # default: false
      namespace: kafka-system       # default: kafka-system
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `false` | Whether the Kafka cluster is deployed |
| `namespace` | string | `kafka-system` | Default namespace value; **not honored at render time** — `kafka-cluster`'s Kustomization and Flux templates hardcode the `kafka-system` namespace regardless of this field (`internal/config/v2/defaults.go`) |

## Dependencies

None enforced by `opencenter cluster service enable|disable`.

## Rendering

`kafka-cluster` has a dedicated descriptor (`internal/services/descriptors/data/service-kafka-cluster.yaml`, `service: kafka-cluster`) covering the Strimzi Kafka custom resource, its Kustomization, and the Strimzi operator's Flux source/Kustomization files. It aggregates into `services-fluxcd-aggregate` and `services-sources-aggregate`.

## CLI commands

```bash
opencenter cluster service enable kafka-cluster
opencenter cluster service disable kafka-cluster
opencenter cluster service status
opencenter cluster service options kafka-cluster
```
