---
id: deployment-profiles
title: "Deployment Profiles"
sidebar_label: Deployment Profiles
description: Recommended service sets for a minimal deployment (no Keycloak) and a full enterprise deployment.
doc_type: how-to
audience: "platform engineers, operators"
tags: [services, profiles, minimal, enterprise, configuration]
---
# Deployment Profiles

**Purpose:** For platform engineers, defines two recommended service sets: a **minimal** profile (core services only, no Keycloak) for lightweight or resource-constrained clusters, and an **enterprise** profile (identity, observability, and backup) for production.

openCenter has no discrete "profile" setting. Which services get deployed is controlled entirely by the per-service `enabled` flags under `opencenter.services`. A profile is therefore just a known-good combination of those flags. This page documents two combinations you can copy into your cluster configuration.

For the full catalog of services, their defaults, and dependencies, see [Platform Services Reference](../reference/platform-services.md). For how to apply and verify changes, see [Customize Services](customize-services.md).

## Choosing a profile

| Consideration | Minimal | Enterprise |
| --- | --- | --- |
| Identity / SSO (Keycloak) | No | Yes |
| Web dashboard (Headlamp) | No (needs Keycloak) | Yes |
| Observability (metrics, logs, traces) | No | Yes |
| Backup and disaster recovery | etcd snapshots only (optional) | Full (Velero + etcd) |
| Policy enforcement (Kyverno) | Yes | Yes |
| GitOps delivery (FluxCD) | Yes | Yes |
| Typical use | Dev, edge, resource-constrained, single-tenant | Production, multi-team, regulated |

Start minimal and enable services as you need them. Enabling a service pulls in its dependencies, so review the [dependency graph](../reference/platform-services.md#service-dependencies) before turning things on.

## Minimal profile (no Keycloak)

The smallest set that still produces a working, GitOps-managed cluster with ingress, TLS, and baseline policy. Identity and observability are intentionally left off.

**Enabled**

| Service | Category | Why it's required |
| --- | --- | --- |
| `calico` | Networking (CNI) | Pod networking. A CNI is mandatory. |
| `gateway-api` | Networking | Ingress CRDs (HTTPRoute, Gateway). |
| `gateway` | Networking | Envoy-based ingress implementation. Depends on `gateway-api`. |
| `cert-manager` | Security | TLS certificate automation for ingress endpoints. |
| `kyverno` | Security | Baseline admission policies. Lightweight; keep it on. |
| `fluxcd` | GitOps | The delivery engine. Reconciles everything else. |
| `sources` | GitOps | FluxCD `GitRepository` sources. Depends on `fluxcd`. |
| provider CSI | Storage | Persistent volumes. `openstack-csi` (+ `openstack-ccm`) or `vsphere-csi`. |

**Disabled**

| Service | Reason |
| --- | --- |
| `keycloak` | Excluded by design in this profile. |
| `headlamp` | Depends on Keycloak for OIDC login. |
| `rbac-manager` | Its main value is Keycloak group-to-RBAC mapping. |
| `postgres-operator` | Only needed as the Keycloak database backend. |
| `olm` | Only needed to run the Keycloak operator. |
| `kube-prometheus-stack`, `loki`, `tempo` | Observability stack. Enable when you need it. |
| `velero`, `etcd-backup`, `external-snapshotter` | Backup stack. See note below. |

> **Backup note:** `etcd-backup` is disabled by default. Enable it only after configuring an absolute `s3_endpoint`, `s3_bucket_name`, `s3_region`, and `secrets.etcd_backup.access_key_id` plus `secret_access_key`. It runs nightly at 01:00 when enabled.

**Configuration**

```yaml
opencenter:
  identity:
    oidc:
      enabled: false          # no Keycloak-backed OIDC in this profile
  services:
    # Core: networking, TLS, policy, GitOps
    calico:
      enabled: true
    gateway-api:
      enabled: true
    gateway:
      enabled: true
    cert-manager:
      enabled: true
      email: "admin@example.com"
    kyverno:
      enabled: true
    fluxcd:
      enabled: true
    sources:
      enabled: true

    # Storage (choose the driver for your provider)
    openstack-csi:
      enabled: true            # OpenStack
    openstack-ccm:
      enabled: true            # OpenStack
    # vsphere-csi:
    #   enabled: true          # VMware instead of the two above

    # Explicitly off
    keycloak:
      enabled: false
    headlamp:
      enabled: false
    rbac-manager:
      enabled: false
    postgres-operator:
      enabled: false
    olm:
      enabled: false
    kube-prometheus-stack:
      enabled: false
    loki:
      enabled: false
    tempo:
      enabled: false
    velero:
      enabled: false
    external-snapshotter:
      enabled: false
    etcd-backup:
      enabled: false          # enable only with endpoint, bucket, region, and secrets configured
```

## Enterprise profile (recommended for production)

Everything in the minimal profile plus identity, RBAC, full observability, backup, and management tooling. Configure `etcd-backup` with its required S3 settings and service-specific secrets before enabling it.

**Enabled (in addition to the minimal core)**

| Service | Category | Purpose |
| --- | --- | --- |
| `keycloak` | Security | OIDC/SAML identity provider. Depends on `cert-manager`, `gateway-api`, `postgres-operator`, `olm`. |
| `postgres-operator` | Management | PostgreSQL backend for Keycloak. |
| `olm` | Management | Runs the Keycloak operator. |
| `rbac-manager` | Security | Maps Keycloak groups to Kubernetes RBAC. |
| `headlamp` | Management | Web dashboard with OIDC login. Depends on `keycloak`. |
| `kube-prometheus-stack` | Observability | Prometheus, Grafana, Alertmanager. |
| `loki` | Observability | Log aggregation. Depends on `kube-prometheus-stack`. |
| `tempo` | Observability | Distributed tracing. Depends on `kube-prometheus-stack`. |
| `velero` | Backup | Application and volume backup. Depends on a CSI driver. |
| `etcd-backup` | Backup | Scheduled etcd snapshots to object storage. |
| `external-snapshotter` | Storage | Volume snapshot controller. Depends on a CSI driver. |

**Configuration**

```yaml
opencenter:
  identity:
    oidc:
      enabled: true
      source: internal
      provider: keycloak
  services:
    # Core: networking, TLS, policy, GitOps
    calico:
      enabled: true
    gateway-api:
      enabled: true
    gateway:
      enabled: true
    cert-manager:
      enabled: true
      email: "admin@example.com"
    kyverno:
      enabled: true
    fluxcd:
      enabled: true
    sources:
      enabled: true

    # Identity and access
    keycloak:
      enabled: true
      hostname: "auth.<org>.<cluster>.<region>.k8s.opencenter.cloud"
    postgres-operator:
      enabled: true
    olm:
      enabled: true
    rbac-manager:
      enabled: true
    headlamp:
      enabled: true
      hostname: "dashboard.<org>.<cluster>.<region>.k8s.opencenter.cloud"

    # Observability
    kube-prometheus-stack:
      enabled: true
      prometheus_volume_size: 100
    loki:
      enabled: true
      volume_size: 50
    tempo:
      enabled: true

    # Storage and backup
    openstack-csi:
      enabled: true            # or vsphere-csi for VMware
    openstack-ccm:
      enabled: true            # OpenStack only
    external-snapshotter:
      enabled: true
    velero:
      enabled: true
    etcd-backup:
      enabled: true
      s3_endpoint: "https://s3.example.com"
      s3_bucket_name: "my-cluster-etcd-backups"
      s3_region: "us-east-1"
    external-snapshotter:
      enabled: true

secrets:
  etcd_backup:
    access_key_id: "<access-key-id>"
    secret_access_key: "<secret-access-key>"
```

## Provider notes

* **OpenStack:** enable `openstack-ccm` and `openstack-csi`. These are on by default for the OpenStack provider.
* **VMware:** enable `vsphere-csi` and disable the `openstack-*` services.
* **Kind / baremetal:** the `openstack-*` services and `velero` are disabled automatically. Kind additionally enables `olm` and `postgres-operator` when Keycloak is in play.

## Apply and verify

After editing your configuration:

```bash
opencenter cluster validate
```

Validation checks for missing required secrets, unmet service dependencies, and configuration conflicts. See [Validate Configuration](validate-configuration.md) and [Customize Services](customize-services.md) for the full workflow.
