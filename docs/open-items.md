---
id: open-items
title: "Open Items"
sidebar_label: Open Items
description: Prioritized remediation plan and register for generator ownership and SOPS key management.
doc_type: reference
audience: "developers, platform engineers"
tags: [known-issues, secrets, sops, gitops, follow-up]
---
# Open Items

**Purpose:** Closure record for Phases 0–6 of generator ownership and SOPS key management. All phases are complete on branch `remediation/generator-ownership-and-sops-keys`; this page records delivered behavior and validation, not open or planned work.

## Context

The earlier review covered `remediation/generator-ownership-and-sops-keys` through commit `dddc9c8`, from `a854cd0` onward. The branch has now completed the remediation. The decisions, implementation outcomes, and final validation below supersede the earlier open/planned status.

## Review corrections and decisions

* The generated command reference now contains **89 generated pages**.
* `RegisterKey` does not auto-promote in the generic path. Reconcile never silently chooses a winner from iteration order.
* Primary selection is explicit through `SetPrimaryKey` and `opencenter secrets keys set-primary`.
* SOPS recipient handling is a security and lifecycle-correctness concern, not a formatting-only issue.
* Secret-artifact routing and generated membership are one topology defect and were implemented together.
* `.sops.yaml` is shared and tracked; its ignore rule was removed.
* The restored `.mise.toml` task definitions and `docs/refactor-audit-report.md` are retained and are not product debt.
* The `custom/` omission for services using verbatim `kustomization_content` is intentional and remains deferred until that mechanism is retired.

## Completed phases

### Phase 0 — Correct and maintain this register

**Status:** complete.

This page is the closure record for the reviewed decisions, implementation outcomes, ownership boundaries, deferred decisions, and final validation. It no longer tracks the remediation as unresolved work.

### Phase 1 — Critical: unblock rotation

**Status:** complete.

Implemented `KeyRegistry.SetPrimaryKey(ctx, cluster, keyType, fingerprint)` and the dedicated `opencenter secrets keys set-primary` command. Selection requires an exact active candidate in the requested cluster and key type, performs one locked read-modify-write, is idempotent, and clears other primary flags in the group. `RegisterKey` performs no implicit promotion, and reconcile promotes only when the complete active candidate set is singular. `UpdateKeys` and `ReplacePrimaryAndArchive` are atomic. Post-mutation ACTIVE Age recipients are used, and revocation plus SSH rotation are transactional.

### Phase 2 — Preserve per-rule SOPS recipient sets

**Status:** complete.

SOPS mutations operate independently on each creation rule, including `FilenameOverride` re-encryption. Recipient sets are not flattened across rules; additions and removals preserve non-target rules and protect against duplicate or empty recipient configurations.

### Phase 3 — Secret routing and conditional membership

**Status:** complete.

Implemented a neutral grouped physical-artifact planner with deterministic merging and conflict rejection. Grafana artifacts route to `services/kube-prometheus-stack/secret.yaml`, while arbitrary `service_secrets` retain same-name routing without making secret ownership depend on GitOps topology. Ownership is represented by hash state; unsafe or legacy state fails closed, with no adoption or deletion of unowned artifacts. Final rendered Kustomization membership is validated, including single-service-scoped validation. Partial-success ownership commits are supported, state-write failure rolls back, and each overlay is a per-overlay `flock` transaction. Services and managed-services scans are sorted and preserve error and symlink safety. Empty managed-services output emits `resources: []`.

### Phase 4 — Generated-overlay Kustomize smoke test

**Status:** complete.

The strict generated Kustomize matrix now has zero allowlist entries and zero failures. The implementation repaired the root `flux-system` reference, Keycloak postgres and operator references, and Kyverno/OLM `BaseOnly` defaults.

### Phase 5 — Documentation generator

**Status:** complete.

The generator uses a deterministic built-in command factory with no external plugin discovery. It preserves frontmatter and applies defaults, prunes stale pages, includes `secrets keys set-primary` and the other previously missing built-ins, and is idempotent. There are currently 89 generated pages.

### Phase 6 — Hygiene and branch-history cleanup

**Status:** complete.

`.sops.yaml` was removed from `.gitignore`. `gofmt` was run cleanly on exactly these six files:

* `internal/cluster/validation_formatter_test.go`
* `internal/config/defaults/registry_test.go`
* `internal/config/v2/defaults_test.go`
* `internal/config/v2/readiness_test.go`
* `internal/core/validation/validators/network_test.go`
* `internal/gitops/copy_test.go`

The restored `.mise.toml` task definitions and `docs/refactor-audit-report.md` were confirmed wanted, retained, and are no longer product debt.

## Deferred by decision

**Hard removal of deprecated renderer keys.** `custom_resources`, `kustomization_content`, `overlay_files_renderer`, `override_values_renderer`, `single_stage`, `base_only`, `source_name`, `kustomization_name`, `override_depends_on`, and `has_override_values` remain supported for compatibility while deprecated. Removing them requires a major schema version bump; the underlying Go fields remain the supported contributor surface — see [Adding Services](contributing/adding-services.md).

**A `generated/` overlay subdirectory.** Rejected in favour of the `.opencenter-generated.json` manifest plus a user-owned `custom/` directory, because introducing the subdirectory would break existing repositories, fixtures, and tests without adding safety the manifest does not already provide.

**`custom/` for verbatim `kustomization_content`.** The omission of a seeded `custom/` directory for services whose overlay is supplied verbatim is intentional and tested. Defer changing it until `kustomization_content` is retired; hand-authored files remain protected by the generated-file manifest, but operators must add their own resource entry in the meantime.

## Completion / outcome summary

* Phases 0–6 are complete on `remediation/generator-ownership-and-sops-keys`.
* Key lifecycle operations are explicit, atomic, and transactionally safe; SOPS recipient rules remain independent.
* Secret artifacts, ownership state, overlay membership, and generated Kustomize output are deterministic and fail closed for unsafe state.
* Built-in command documentation is deterministic, frontmatter-compliant for the applicable corpus, stale-free, and idempotent.
* The only remaining frontmatter findings are the eleven unrelated pre-existing pages listed in Final validation; they were not changed.

## Final validation

* **PASS** — `go test ./internal/sops ./internal/secrets ./internal/secretartifacts ./internal/gitops ./cmd -count=1`.
* **PASS** — `go build ./...`.
* **PASS** — `go vet ./...`.
* **PASS** — `mise run test`.
* **PASS** — `mise run test-race`.
* **PASS** — `mise run godog`.
* **PASS** — `go test -tags tools ./cmd/docs -count=1`.
* **PASS** — strict generated-overlay Kustomize matrix: zero allowlist entries and zero failures.
* **PASS** — `git diff --check`.
* **PASS** — final Oracle review.
* **PASS** — docs generation run twice; both runs produced the identical `docs/reference` diff SHA-256 `5c369101e63979eaedf0a2b3681a5155f8c6308d0bb3335726e35c1336754737`.
* **NON-PASS (known baseline only)** — strict frontmatter audit reports exactly these 11 unrelated pre-existing pages: `docs/architecture.md`, `docs/dead-code-removal-report.md`, `docs/deduplication-report.md`, `docs/library-extraction-report.md`, `docs/llm-code-map.md`, `docs/operations/manage-worker-pools.md`, `docs/refactor-audit-report.md`, `docs/reference/services/README.md`, `docs/render-remediation.md`, `docs/work/openCenter-cli-cluster-deployment-plan.md`, and `docs/work/openCenter-documentation-discrepancies.md`. Excluding only those paths, 196 files pass. None of these eleven pages was fixed as part of this remediation.

## Related

* [Manage Secrets](operations/manage-secrets.md) — key lifecycle, reconcile, and revocation guidance
* [Security Model](concepts/security-model.md) — key lifecycle invariants
* [Secrets Management Codemap](CODEMAPS/secrets-management.md) — package structure and behaviors
* [Adding Services](contributing/adding-services.md) — generator ownership and BaseConfig fields
