---
id: providers-map
title: "Explain Provider Capability Boundaries"
sidebar_label: Providers
description: Distinguishes provider configuration, generation, bootstrap, drift detection, and destruction capabilities without conflating planned providers with supported implementations.
doc_type: explanation
audience: "contributors, maintainers, operators"
tags: [providers, openstack, vmware, baremetal, kind]
---
# Providers

Provider support is split across three boundaries: configuration and generation in `internal/config/v2` and `internal/gitops`, lifecycle bootstrap in `internal/cluster`, and cloud-state drift interfaces in `internal/cloud`. A provider may participate in one boundary without implementing another.

## Capability matrix

| Provider | Config/generate | Bootstrap/deploy | Drift provider | Current boundary |
|---|---:|---:|---:|---|
| OpenStack | Yes | Yes | Yes | `internal/cloud/openstack`, shared infrastructure bootstrap, OpenTofu |
| VMware/vSphere | Yes | Yes | Yes | `internal/cloud/vmware`, shared infrastructure bootstrap with vSphere credentials |
| Baremetal | Yes | Yes | No cloud drift implementation | Shared infrastructure bootstrap with static-node validation; no OpenStack/vSphere credentials |
| Kind | Yes | Yes | No | `internal/cloud/kind` lifecycle plus `kindBootstrapProvider`; local development integration |
| AWS | Config types may exist | Rejected by active bootstrap routing | Not registered as a supported drift provider | Planned/unavailable |
| GCP | Config types may exist | Rejected by active bootstrap routing | Not registered as a supported drift provider | Planned/unavailable |
| Azure | Config types may exist | Rejected by active bootstrap routing | Not registered as a supported drift provider | Planned/unavailable |

The active bootstrap validator rejects providers outside OpenStack, VMware, Baremetal, and Kind.

## Drift interface

`internal/cloud.CloudProvider` defines:

```go
GetCurrentState(ctx, cfg) (*InfrastructureState, error)
DetectDrift(ctx, desired, actual) (*DriftReport, error)
ReconcileDrift(ctx, drift) error
```

`CloudProviderFactory` is a registry for drift-capable implementations. It is separate from lifecycle deploy providers: Kind deployment is wired directly into cluster lifecycle, while OpenStack and VMware expose provider APIs for state comparison and reconciliation.

## Bootstrap routing

`internal/cluster/bootstrap_provider.go` defines the lifecycle provider contract:

```go
BuildSteps(cfg, clusterPaths, opts) ([]bootstrapStep, error)
```

`openstackBootstrapProvider` is shared by OpenStack, VMware, and Baremetal. `buildProviderBootstrapEnvironment` extracts only the credentials relevant to the selected provider and validates prerequisites. `kindBootstrapProvider` handles Kind-specific create/readiness and local Flux steps. Bootstrap state makes these ordered plans resumable through `--step` and `--from-step`.

## Supporting packages

| Package | Boundary |
|---|---|
| `internal/credentials` | Extract provider credentials from validated configuration; it does not deploy resources |
| `internal/tofu` | Invoke OpenTofu/Terraform for infrastructure provisioning where the lifecycle path requires it |
| `internal/cloud/openstack` | OpenStack discovery, state collection, drift comparison, and supported reconciliation |
| `internal/cloud/vmware` | vSphere state and drift implementation |
| `internal/cloud/kind` | Kind create/delete/readiness and kubeconfig operations |
| `internal/cluster/orchestration` | Guided provider configuration and capability handlers |
| `internal/localdev` | Local Kind/Gitea/Flux workflow services, not a cloud provider abstraction |

## Related maps

- [Cluster lifecycle](cluster-lifecycle.md) — bootstrap and destroy callers
- [Config system](config-system.md) — provider config and validation
- [Import, operations, and resilience](import-operations-and-resilience.md) — drift and backup operations
