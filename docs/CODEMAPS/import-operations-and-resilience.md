---
id: import-operations-and-resilience
title: "Explain Import Operations and Resilience"
sidebar_label: Import and Resilience
description: Explains repository import scan/apply/report behavior, drift and backup operations, and the lock, retry, and circuit-breaker controls that protect mutating workflows.
doc_type: explanation
audience: "contributors, maintainers, operators"
tags: [import, operations, drift, backup, resilience]
---
# Import, operations, and resilience

These packages support existing-cluster adoption and day-2 operation without owning the primary configuration or GitOps rendering paths.

## Import flow

```text
cluster import scan --repo-path <repo>
  -> internal/importer.Scanner.ScanRepo
       discover cluster directories, overlays, README, and legacy configs
       infer provider, organization, topology, services, and fields
       attach evidence, confidence, conflicts, and skipped fields
  -> saved scan artifact
cluster import report
  -> importer.RenderScanResult (text/json/yaml)
cluster import apply
  -> SelectApprovedFields
  -> protected/conflicted/low-confidence fields skipped
  -> explicit YAML patch and diff
```

`internal/importer` owns discovery, detectors, namespace overrides, artifact storage, protected fields, YAML patching, and report rendering. High-confidence, non-conflicting fields can be selected; protected or ambiguous values remain for manual review. Import does not silently overwrite an existing configuration.

## Operations

| Area | Interface/package | Boundary |
|---|---|---|
| Drift | `internal/operations.DriftDetector` | Load desired config, query a provider, produce severity/reconcilable drift, optionally reconcile or schedule checks |
| Provider state | `internal/cloud.CloudProvider` | Retrieve provider state and perform provider-specific drift comparison/reconciliation |
| Backup | `internal/operations.BackupManager` | Archive config, encrypted Age/SSH material, GitOps state, and Terraform state; list, restore, delete, and schedule backups |
| Lifecycle locks | `internal/resilience.LockManager` | Prevent concurrent operations over a resource |

Backup archives are checksummed and can be encrypted with AES-256-GCM. Restore verifies integrity before extraction. Backup retention and scheduling are operation-level concerns, not config or renderer concerns.

## Resilience controls

`internal/resilience` provides:

- `LockManager` with file or Redis backends, TTLs, metadata, refresh, inspection, and force-break operations;
- `RetryHandler` with bounded attempts, exponential backoff, maximum delay, jitter, context cancellation, and retryable-error policy; and
- `CircuitBreaker` with closed/open/half-open states, failure and success thresholds, timeout, and limited half-open requests.

Lifecycle and other mutating command paths can combine these controls with their own persisted bootstrap state. A lock coordinates ownership; bootstrap state coordinates step progress; retry and circuit-breaker policy coordinates transient dependency failures. They are complementary, not interchangeable.

## Security and filesystem boundaries

Import patches and backup extraction must use validated paths and atomic filesystem helpers. External commands used by operations pass through `internal/security.CommandRunner`; audit logging and credential masking remain cross-cutting controls. Shared path and validation primitives are in `internal/core/paths` and `internal/core/validation`.

## Related maps

- [Cluster lifecycle](cluster-lifecycle.md) — bootstrap state and lifecycle callers
- [Providers](providers.md) — provider drift versus deploy capabilities
- [Config system](config-system.md) — validated target configuration
- [Runtime extensions and local development](runtime-extensions-and-local-development.md) — security and command execution boundaries
