---
id: secrets-management-map
title: "Explain Secrets Management Boundaries"
sidebar_label: Secrets Management
description: Explains logical secret ownership, encrypted manifest synchronization, SOPS overlay values, key lifecycle state, and the boundaries between these workflows.
doc_type: explanation
audience: "contributors, maintainers, operators"
tags: [secrets, sops, age, encryption, ownership]
---
# Secrets management

Secret handling has two related but distinct flows:

1. `internal/secrets` synchronizes configuration secrets into encrypted Kubernetes manifests and tracks ownership/state.
2. `internal/sops` encrypts provider and GitOps overlay values and manages SOPS/Age file operations.

`internal/secretartifacts` plans the physical manifest targets before either renderer or backend performs materialization.

## Manifest synchronization flow

```text
v2.Config.Secrets
  -> secretartifacts.Plan
       logical owners -> normalized target service/path -> merged payload
  -> secrets.DefaultSecretsManager.SyncSecrets
       acquire per-overlay lock
       load prior ownership state
       refuse unsafe/unowned adoption
       encrypt/write each physical manifest
       journal mutations and reconcile stale artifacts
       persist ownership hashes/state
       audit result
```

`secretartifacts.Plan` maps logical owners to physical artifacts. For example, Grafana data targets the `kube-prometheus-stack` artifact and multiple owners may merge only when normalized keys do not conflict. Planning is deterministic and validates target services.

## Transaction and drift behavior

`internal/secrets/manager.go` provides locked transactional sync. It snapshots files, journals mutations, writes encrypted output, records ownership hashes, and rolls back through the rollback window when a mutation fails. Validation decrypts manifests and compares them with config, reporting drift, missing manifests, orphaned artifacts, and security issues.

Ownership state is separate from payload data. It prevents adopting an existing unowned or unsafe file and allows stale generator-owned artifacts to be identified without treating arbitrary user files as generated output.

## SOPS overlay flow

```text
cluster config
  -> SOPS manager selects provider/GitOps overlay files
  -> Age/SOPS encryptor writes encrypted values
  -> generated overlay promotion
```

This is not the same as secret manifest sync. The SOPS manager handles file-level encryption, `.sops.yaml` generation/validation, Age key storage, and encryption checks. `SetupService` uses an overlay encryption hook before promoting generated application output.

## OpenStack storage credential boundary

`cluster service storage plan/apply` is a separate one-service provisioning path. [`internal/cluster/storage/openstack`](../../internal/cluster/storage/openstack) plans Swift application credentials or project-scoped EC2 credentials, container/bucket actions, typed service wiring, and recovery state. It does not encrypt Kubernetes manifests, update secret-artifact ownership, or run the SOPS manager. Existing complete credential pairs are reused unless `--rotate-credentials` is supplied; dry-run performs preflight only and does not invoke remote mutations.

The resulting credential values can later be consumed by rendering and secret-artifact workflows, but provisioning them here does not itself reconcile the encrypted manifest tree.

## CLI boundary

The production command surface groups key lifecycle under `secrets keys`: `generate`, `rotate`, `backup`, `validate`, `check`, `revoke`, `reconcile`, and `set-primary`. Manifest operations are `secrets sync` and `secrets validate`; file operations are `secrets encrypt`, `decrypt`, and `status`.

Backend-oriented commands (`login`, `list`, `describe`, `get`, `set`, `delete`) are separate from SOPS manifest synchronization. Do not collapse backend CRUD, SOPS file encryption, and generated Kubernetes manifest sync into one lifecycle.

## Package ownership

| Package | Responsibility |
|---|---|
| `internal/secretartifacts` | Pure logical-owner to physical-artifact planning, normalization, deterministic merge, and target validation |
| `internal/secrets` | Sync, validation, key registry, rotation/revocation, reconciliation, hooks, multi-cluster coordination, rollback, and audit calls |
| `internal/cluster/storage/openstack` | OpenStack storage mappings, credential creation/reuse/rotation, recovery, and typed config persistence; not encrypted manifest sync |
| `internal/sops` | SOPS/Age encryption engine, key manager, overlay encryption, and repository SOPS configuration |
| `internal/barbican` | OpenStack Key Manager client used by backend-specific secret flows |
| `internal/security` | Credential masking, command sanitization, and audit primitives used by secret operations |

## Related maps

- [Rendering ownership and secret artifacts](rendering-ownership-and-secret-artifacts.md) — planner/renderer contract
- [OpenStack provider and storage operations](openstack-provider-storage-operations.md) — provider and storage credential boundary
- [GitOps engine](gitops-engine.md) — overlay encryption during generation
- [Config system](config-system.md) — secret input model and paths
- [CLI commands](cli-commands.md) — command registration
