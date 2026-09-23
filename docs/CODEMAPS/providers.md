---
id: providers-map
title: "Explain Provider Capability Boundaries"
sidebar_label: Providers
description: Distinguishes provider configuration, generation, bootstrap, drift detection, and destruction capabilities without conflating planned providers with supported implementations.
doc_type: explanation
audience: "contributors, maintainers, operators"
tags: [providers, openstack, magnum, vmware, baremetal, kind]
---
# Providers

Provider support is split across three boundaries: configuration and generation in `internal/config/v2` and `internal/gitops`, lifecycle bootstrap in `internal/cluster`, and cloud-state drift interfaces in `internal/cloud`. A provider may participate in one boundary without implementing another.

## Capability matrix

| Provider | Config/generate | Bootstrap/deploy | Drift provider | Provider/storage operations | Current boundary |
|---|---:|---:|---:|---:|---|
| OpenStack | Yes | Yes | Yes | Yes | `internal/cluster/provider/openstack`, `internal/cluster/storage/openstack`, `internal/cloud/openstack`, shared infrastructure bootstrap, OpenTofu |
| Magnum | Yes | Yes | No | No | `internal/cloud/magnum` (standalone Magnum API client) plus `internal/cluster/magnum_bootstrap_provider.go`, `magnum_destroy_provider.go`, `magnum_configure_orchestrator.go`; configuration at `opencenter.infrastructure.cloud.magnum`; no OpenTofu or drift implementation |
| VMware/vSphere | Yes | Yes | Yes | No | `internal/cloud/vmware`, shared infrastructure bootstrap with vSphere credentials |
| Baremetal | Yes | Yes | No cloud drift implementation | No | Shared infrastructure bootstrap with static-node validation; no OpenStack/vSphere credentials |
| Kind | Yes | Yes | No | No | `internal/cloud/kind` lifecycle plus `kindBootstrapProvider`; local development integration |
| AWS | Yes (schema and validator accept it; `internal/cluster/bootstrap_service.go` has a latent Terraform-based step path) | Blocked at the CLI layer | Not registered as a supported drift provider (no `internal/cloud/aws` package) | No | Planned/unavailable to end users |
| GCP | Yes (schema and validator accept it; same latent bootstrap path as AWS) | Blocked at the CLI layer | Not registered as a supported drift provider (no `internal/cloud/gcp` package) | No | Planned/unavailable to end users |
| Azure | Yes (schema and validator accept it; same latent bootstrap path as AWS) | Blocked at the CLI layer | Not registered as a supported drift provider (no `internal/cloud/azure` package) | No | Planned/unavailable to end users |

`internal/config/v2/readiness.go` lists `openstack, aws, gcp, azure, baremetal, vsphere, vmware, kind, magnum` as schema-valid provider strings, and `internal/config/v2/provider.go` has a real validator for each of `openstack`/`aws`/`gcp`/`azure`. `internal/cluster/bootstrap_service.go` even contains a `case "aws", "gcp", "azure":` branch that builds real `make terraform` / `terraform apply` bootstrap steps. None of this is reachable through the CLI: `cmd/provider_availability.go:checkProviderAvailability` is called from `cluster init`, `cluster generate`, and `cluster deploy` and unconditionally rejects `aws`, `gcp`, and `azure` with "provider ... is planned for a future release and not yet available. Supported providers: openstack, vmware, kind, baremetal" (that message omits Magnum, which is reachable only by explicitly setting `opencenter.infrastructure.provider: magnum`, not offered as a `cluster init --type` choice in that error text). Treat the internal aws/gcp/azure code paths as unused scaffolding, not a supported capability.

## Drift interface

`internal/cloud.CloudProvider` defines:

```go
GetCurrentState(ctx, cfg) (*InfrastructureState, error)
DetectDrift(ctx, desired, actual) (*DriftReport, error)
ReconcileDrift(ctx, drift) error
```

`CloudProviderFactory` is a registry for drift-capable implementations. It is separate from lifecycle deploy providers: Kind deployment is wired directly into cluster lifecycle, while OpenStack and VMware expose provider APIs for state comparison and reconciliation.

OpenStack provider and storage operations are separate from the `CloudProvider` drift interface and lifecycle deployment. Provider plan/apply uses `internal/cloud/openstack` read-only discovery and local typed persistence. Storage plan/apply uses the storage adapter for explicit container and credential actions for one service, then persists typed configuration with recovery semantics. Neither operation provisions the cluster itself.

## Bootstrap routing

`internal/cluster/bootstrap_provider.go` defines the lifecycle provider contract:

```go
BuildSteps(cfg, clusterPaths, opts) ([]bootstrapStep, error)
```

`openstackBootstrapProvider` is shared by OpenStack, VMware, and Baremetal. `buildProviderBootstrapEnvironment` extracts only the credentials relevant to the selected provider and validates prerequisites. `kindBootstrapProvider` handles Kind-specific create/readiness and local Flux steps. `newMagnumBootstrapProvider` (in `internal/cluster/magnum_bootstrap_provider.go`) has a separate managed-provider lifecycle built on `internal/cloud/magnum`: deploy creates and polls a cluster from an existing Magnum cluster template, then securely writes kubeconfig; `newMagnumDestroyProvider` deletes the Magnum cluster. Magnum does not invoke OpenTofu. Bootstrap state makes these ordered plans resumable through `--step` and `--from-step`.

## Supporting packages

| Package | Boundary |
|---|---|
| `internal/credentials` | Extract provider credentials from validated configuration; it does not deploy resources |
| `internal/tofu` | Invoke OpenTofu/Terraform for infrastructure provisioning where the lifecycle path requires it |
| `internal/cloud/openstack` | `clouds.yaml` profile loading, read-only provider discovery, storage preflight, and credential/container adapters |
| `internal/cloud/magnum` | Standalone Magnum API client (create/get/wait-visible/wait-ready/export-kubeconfig/delete/wait-deleted a Magnum-managed cluster); Gophercloud v1's Magnum operations don't accept a context, so the client checks cancellation before and after each call and re-authenticates its own `ProviderClient` per request group |
| `internal/cluster/provider/openstack` | Typed provider planning and local atomic persistence with no remote actions |
| `internal/cluster/storage/openstack` | One-service storage mappings, credential planning, remote-action sequencing, and recovery-aware persistence |
| `internal/cloud/vmware` | vSphere state and drift implementation |
| `internal/cloud/kind` | Kind create/delete/readiness and kubeconfig operations |
| `internal/cluster/orchestration` | Guided provider configuration and capability handlers |
| `internal/localdev` | Local Kind/Gitea/Flux workflow services, not a cloud provider abstraction |

## Related maps

- [Cluster lifecycle](cluster-lifecycle.md) — bootstrap and destroy callers
- [OpenStack provider and storage operations](openstack-provider-storage-operations.md) — typed provider planning and explicit storage provisioning
- [Config system](config-system.md) — provider config and validation
- [Import, operations, and resilience](import-operations-and-resilience.md) — drift and backup operations
