---
id: open-items
title: "Open Items"
sidebar_label: Open Items
description: Known outstanding issues and follow-up work from the generator ownership and SOPS key management remediation.
doc_type: reference
audience: "developers, platform engineers"
tags: [known-issues, secrets, sops, gitops, follow-up]
---
# Open Items

**Purpose:** Register of known outstanding issues left after the generator file-ownership and SOPS key-management remediation, with evidence and recommended fixes so each can be picked up independently.

Context: the remediation landed on `remediation/generator-ownership-and-sops-keys` as eight commits from `a854cd0` to `dddc9c8`. At the time of writing that branch passes `go build ./...`, `go vet ./...`, `mise run test`, `mise run test-race` (46 packages), and `mise run godog`.

## 1. Rotation dead-end on multi-recipient clusters

**Severity:** blocks rotation. Fix before relying on the branch.

A cluster with two or more active Age keys and no primary cannot rotate, and the CLI provides no way to recover.

The chain:

* `DefaultKeyRegistry.RegisterKey` (`internal/secrets/registry.go`) rejects a second active primary but never promotes one when none exists.
* `normalizeRegistryPrimaries` (`internal/secrets/registry.go`) deliberately refuses to guess when a cluster has two or more active keys and no unique `RotatedFrom` leaf, so the group is left with no primary.
* `DefaultKeyReconciler.Reconcile` with `apply` imports every recipient as `Primary: false`.
* No command promotes a primary. There is no match for a promote or set-primary subcommand anywhere under `cmd/`.
* `RotateAgeKey` and `RotateSSHKey` (`internal/secrets/rotation.go`) require `GetPrimaryKey` and fail with a message stating that a reconcile is needed. That advice is wrong: reconcile cannot set a primary.

**Reachability:** this is the exact scenario reconcile exists for. A registry missing recipients, a `.sops.yaml` holding three, then `opencenter secrets keys reconcile --cluster <name> --apply` produces three active non-primary entries and rotation becomes impossible.

**Regression:** on `main`, `GetKey` returned the first active key and rotation proceeded. The primary requirement introduced the dead-end.

**Recommended fix:**

1. In `RegisterKey`, promote the entry when its status is active and no active primary exists for that cluster and key type. This matches `MockKeyRegistry` and makes reconcile's first import the primary, resolving the zero-to-one case automatically.
2. Add explicit promotion for registries that are already ambiguous, either as a `--promote-primary <fingerprint>` flag on reconcile or a small dedicated subcommand. Do not infer a winner.
3. Correct the rotation error text to name the actual remedy.

## 2. The registry conformance suite does not cover primary semantics

**Severity:** allows issue 1 and its class to recur silently.

`internal/secrets/registry_conformance_test.go` runs nine subtests against both `DefaultKeyRegistry` and `MockKeyRegistry`, covering fingerprint uniqueness across statuses, metadata defaults, multiple active keys, not-found lookups, fingerprint-targeted update, and cluster filtering. None of them assert anything about primary selection; the file contains no reference to primary at all.

Consequently the two implementations diverge today: `MockKeyRegistry.RegisterKey` promotes the first active key when no primary exists, and `DefaultKeyRegistry.RegisterKey` does not. Mock-backed rotation tests can therefore pass against behavior the real registry refuses. This is precisely the failure mode the suite was introduced to prevent — the original single-active-key contradiction shipped because mock-backed tests permitted what the real registry rejected.

**Recommended fix:** add subtests asserting at most one active primary per cluster and key type, deterministic `GetPrimaryKey`, the promote-on-first-active rule once decided, `ReplacePrimary` demoting the predecessor and promoting the successor in one write, and `GetKey` falling back to the earliest active key when no primary is set. Then reconcile the two implementations against them.

## 3. `.sops.yaml` is git-ignored while still tracked

**Severity:** latent.

`.gitignore` matches `.sops.yaml`. Git skips ignore rules for tracked files, so nothing is broken today and `git check-ignore` only reports the match with `--no-index`. The risk is that if the file is ever untracked, the repaired version silently stops being shared, and the invalid-YAML failure it fixed returns for everyone. Either untrack it deliberately and document that each checkout supplies its own, or drop the ignore entry.

## 4. SOPS rewrites flatten per-rule recipient sets

**Severity:** low, pre-existing behavior preserved.

`rewriteSOPSAgeValues` (`internal/secrets/sops_yaml.go`) writes the same joined recipient list to every creation rule that has an `age` key. The previous implementation did the same, so this is not a regression, but it silently collapses configurations that intentionally scope different recipients to different paths. If per-path recipient scoping is meant to be supported, rules need to be updated independently.

## 5. Command reference documentation cannot be regenerated

**Severity:** medium for documentation accuracy.

`mise run docs-gen` (`go run cmd/docs/generate.go`) strips the Diátaxis front-matter — `id`, `title`, `sidebar_label`, `description`, `doc_type`, `audience`, `tags` — from all 99 pages under `docs/reference/opencenter/`, and reflows lists. Those keys are required by `hack/scripts/audit_doc_frontmatter.py`, so the generator's raw output does not satisfy the project's own documentation rule. Some post-processing step must exist that produced the committed state, but it is not in the repository.

As a result these commands have no reference page: `secrets keys reconcile` (new), `cluster migrate-layout` (new), and `cluster pool`, `settings`, `cluster sync`, which were already missing beforehand.

**Recommended fix:** make the generator emit compliant front-matter, then regenerate and commit all pages in one reviewable change.

## 6. Unformatted test files on `main`

**Severity:** cosmetic.

These files are not `gofmt`-clean on `main` and were deliberately left untouched to keep review diffs readable:

* `internal/cluster/validation_formatter_test.go`
* `internal/config/defaults/registry_test.go`
* `internal/config/v2/defaults_test.go`
* `internal/config/v2/readiness_test.go`
* `internal/core/validation/validators/network_test.go`
* `internal/gitops/copy_test.go`

Worth a single formatting commit that touches nothing else.

## 7. Needs a decision: two reverted deletions

While committing, the working tree contained two changes that were outside the scope of any task and were reverted rather than committed:

* `.mise.toml` had the `docs-gen` and `tag-wip-failures` task definitions deleted.
* `docs/refactor-audit-report.md` was deleted.

Both were restored. If either deletion was intentional, it needs redoing as a deliberate change.

## Deferred by decision

These were considered and consciously not done, recorded so they are not rediscovered as bugs.

**Hard removal of deprecated renderer keys.** `custom_resources`, `kustomization_content`, `overlay_files_renderer`, `override_values_renderer`, `single_stage`, `base_only`, `source_name`, `kustomization_name`, `override_depends_on`, and `has_override_values` are marked deprecated in the schema and warn when set in user YAML, but still load. Deleting them requires a major schema version bump. The underlying Go fields remain the supported contributor surface — see [Adding Services](contributing/adding-services.md).

**A `generated/` overlay subdirectory.** Rejected in favour of the `.opencenter-generated.json` manifest plus a user-owned `custom/` directory, because introducing the subdirectory would have broken every existing repository, fixture, and test without adding safety the manifest does not already provide.

## Related

* [Manage Secrets](operations/manage-secrets.md) — key lifecycle, reconcile, and revocation guidance
* [Security Model](concepts/security-model.md) — key lifecycle invariants
* [Secrets Management Codemap](CODEMAPS/secrets-management.md) — package structure and behaviors
* [Adding Services](contributing/adding-services.md) — generator ownership and BaseConfig fields
