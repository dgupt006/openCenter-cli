---
id: services-index
title: "Platform Services"
sidebar_label: Services
description: Directory of every platform service configurable in openCenter clusters, with default state and links to per-service reference pages.
doc_type: reference
audience: "operators, platform engineers"
tags: [services, platform, reference]
---

# Platform services

> **Purpose:** For operators and platform engineers, indexes every service defined in `schema/opencenter-v2.schema.json`, organized by category, with its default enabled state and a link to its configuration reference.

Services are configured under `opencenter.services.<name>` (or `opencenter.managed_services.<name>` / `opencenter.managed-service.<name>` for managed-service deployments — both accept the same fields). See [Platform services architecture](../platform-services.md) for how these maps are validated and rendered.

"Default" below reflects `internal/config/v2/defaults.go`. Services marked **Opt-in** have no entry in the default generated configuration at all — you must add them explicitly.

## Networking

| Service | Default | Description |
|---------|---------|-------------|
| [calico](calico.md) | Enabled (`calico-system`) | Calico CNI: pod networking and NetworkPolicy enforcement |
| [cilium](cilium.md) | Opt-in | eBPF-based CNI, alternative to Calico |
| [kube-ovn](kube-ovn.md) | Opt-in | OVN-based overlay CNI, alternative to Calico/Cilium |
| [gateway-api](gateway-api.md) | Enabled (`envoy-gateway-system`) | Gateway API CRDs consumed by the `gateway` service |
| [gateway](gateway.md) | Enabled (`gateway`) | Envoy Gateway implementation of the Gateway API |
| [metallb](metallb.md) | Disabled (`metallb-system`) | Bare-metal LoadBalancer IP address management |

## Security

| Service | Default | Description |
|---------|---------|-------------|
| [cert-manager](cert-manager.md) | Enabled (`cert-manager`) | ACME/self-signed/CA certificate issuance |
| [keycloak](keycloak.md) | Enabled (`keycloak`) | OIDC identity provider, backed by postgres-operator |
| [kyverno](kyverno.md) | Enabled (`kyverno`) | Kubernetes policy engine |
| [rbac-manager](rbac-manager.md) | Enabled (`rbac-system`) | Declarative RBAC via CRDs |
| [sealed-secrets](sealed-secrets.md) | Disabled (`sealed-secrets`) | Asymmetric-encrypted Secret objects |

## Storage

| Service | Default | Description |
|---------|---------|-------------|
| [openstack-ccm](openstack-ccm.md) | Enabled (`openstack-ccm`) | OpenStack Cloud Controller Manager |
| [openstack-csi](openstack-csi.md) | Enabled (`openstack-csi`) | OpenStack Cinder CSI driver |
| [vsphere-csi](vsphere-csi.md) | Disabled (`vmware-system-csi`) | VMware vSphere CSI driver |
| [longhorn](longhorn.md) | Disabled (`longhorn-system`) | Distributed block storage |
| [external-snapshotter](external-snapshotter.md) | Enabled (`external-snapshotter`) | VolumeSnapshot CRDs and controller |

## Observability

| Service | Default | Description |
|---------|---------|-------------|
| [kube-prometheus-stack](kube-prometheus-stack.md) | Enabled (`observability`) | Prometheus, Grafana, Alertmanager |
| [loki](loki.md) | Enabled (`observability`) | Log aggregation, S3 or Swift storage |
| [tempo](tempo.md) | Enabled (`observability`) | Distributed tracing, S3 storage |
| [mimir](mimir.md) | Disabled (`observability`) | Long-term metrics storage |
| [opentelemetry-kube-stack](opentelemetry-kube-stack.md) | Disabled (`observability`) | OpenTelemetry collectors |
| [alert-proxy](alert-proxy.md) | Opt-in (managed service) | Alertmanager webhook forwarding proxy |

## GitOps

| Service | Default | Description |
|---------|---------|-------------|
| [fluxcd](fluxcd.md) | Enabled (`flux-system`) | GitOps reconciliation engine (structural/core) |
| [sources](sources.md) | Enabled (`flux-system`) | Aggregate Flux `GitRepository`/`OCIRepository` sources consumed by every other service's Kustomization |
| [weave-gitops](weave-gitops.md) | Disabled (`flux-system`) | Web dashboard for Flux resources |

## Backup

| Service | Default | Description |
|---------|---------|-------------|
| [velero](velero.md) | Enabled (`velero`) | Cluster resource and volume backup |
| [etcd-backup](etcd-backup.md) | Disabled (`kube-system`) | Nightly etcd snapshot upload to S3-compatible storage |

## Management

| Service | Default | Description |
|---------|---------|-------------|
| [headlamp](headlamp.md) | Enabled (`headlamp`) | Kubernetes dashboard with OIDC login |
| [olm](olm.md) | Enabled (`olm`) | Operator Lifecycle Manager |
| [postgres-operator](postgres-operator.md) | Enabled (`postgres-operator`) | Zalando PostgreSQL operator; required by keycloak |
| [harbor](harbor.md) | Disabled (`harbor`) | Container registry |
| [kafka-cluster](kafka-cluster.md) | Disabled (`kafka-system`) | Apache Kafka via the Strimzi operator |

## Common operations

```bash
# List all service states
opencenter cluster service status

# Enable/disable a service (add --managed for opencenter.managed_services)
opencenter cluster service enable <service> [--param key=value] [--secret key=value]
opencenter cluster service disable <service>

# View a service's example parameters/secrets
opencenter cluster service options <service>
```

## Storage backend defaults

`loki`, `tempo`, and `velero` accept a `storage_type`. `internal/config/services/provider_registry.go` maps the cluster's infrastructure provider to a default backend when one is not set explicitly:

| Infrastructure provider | Default `storage_type` |
|--------------------------|-------------------------|
| OpenStack | `swift` (`s3` also compatible) |
| AWS | `s3` |
| GCP | `gcs` |
| Azure | `azure` |
| Bare-metal / vSphere | `s3` |

Tempo does not actually support a Swift backend at runtime (see [Tempo](tempo.md)); use `s3` against a Swift S3-compatible endpoint on OpenStack.

## Related documentation

- [Platform services architecture](../platform-services.md) — how the descriptor and render-catalog systems render these services, and what the CLI enforces at enable/disable time.
