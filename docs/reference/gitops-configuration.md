---
id: gitops-configuration
title: "GitOps Configuration Reference"
sidebar_label: GitOps Config
description: Complete field reference for opencenter.gitops.* and the related secrets.overlay_units.*, verified against internal/config/v2/config.go and internal/config/overlay/types.go.
doc_type: reference
audience: "platform engineers, operators"
tags: [gitops, configuration, fluxcd, repository, auth]
---
# GitOps Configuration Reference

**Purpose:** For platform engineers and operators, documents every field under `opencenter.gitops` and the paired `secrets.overlay_units` block, with types, defaults, and validation rules, verified against the Go structs that define them (`internal/config/v2/config.go`, `GitOpsConfig`).

## `opencenter.gitops.repository` (required)

| Field | Type | Required | Notes |
| --- | --- | --- | --- |
| `url` | string | yes | Must be a valid URL (`validate:"required,url"`). SSH (`ssh://` / `git@...`) or HTTPS. |
| `branch` | string | no | Target branch, default `main` in practice. |
| `path` | string | no | Directory within the repo for this cluster's manifests. |
| `local_dir` | string | no | Local checkout directory. Written/resolved by `cluster init` (see [Cluster Init Details](../contributing/cluster-init-details.md)); `cluster generate`/`deploy` require this to be set. |
| `secret_name` | string | no | Kubernetes Secret name FluxCD uses for repository access. |

## `opencenter.gitops.base_repo` (optional)

Upstream template repository settings -- distinct from the cluster's own repository above.

| Field | Type | Notes |
| --- | --- | --- |
| `url` | string | Base GitOps-templates repository URL (`validate:"omitempty,url"`). |
| `release` | string | Version tag to pin (e.g. `v0.1.0`). |
| `branch` | string | Branch to track, as an alternative to `release`. |

## `opencenter.gitops.auth`

Exactly one of `ssh` or `token` should be configured for a given repository; [readiness validation](validation-rules.md) rejects configuring both.

**`ssh`** (`GitOpsSSHAuth`): `private_key` (path to the private key file), `public_key` (path to the public key file). Both required when using SSH auth.

**`token`** (`GitOpsTokenAuth`):

| Field | Type | Required | Notes |
| --- | --- | --- | --- |
| `provider` | string | yes | One of `github`, `gitlab`, `gitea`. Readiness validation requires this to match the repository URL's host (`github.com` -> `github`; a GitLab host -> `gitlab`; any other HTTPS host -> `gitea`). |
| `token` | string | one of `token`/`token_file` | Inline access token value. |
| `token_file` | string | one of `token`/`token_file` | Path to a file containing the token; required for bootstrap when using token auth. |
| `owner` | string | no | Repository owner; extracted from the URL if empty. |
| `organization` | string | no | Git organization used as the username in authenticated HTTPS URLs (`https://<organization>:<token>@host/path`). |
| `personal` | bool | no | Default `false`. GitHub-only: when `true`, `flux bootstrap github` is invoked with `--personal` instead of the organization-owned form. |

## `opencenter.gitops.flux`

| Field | Type | Default | Notes |
| --- | --- | --- | --- |
| `interval` | string | -- | FluxCD reconciliation interval override. |
| `prune` | bool | -- | Whether FluxCD prunes resources removed from Git. Every generated service Kustomization sets `prune: true` regardless (see [Renderer Contract](../contributing/rendering-contract.md)); this top-level flag is a config-surface value, not necessarily what every generated manifest uses -- verify against the rendered output for your services. |

## `opencenter.gitops.overlay_units` (`internal/config/overlay/types.go`, `UnitsConfig`)

Stable as of schema version 2.0 -- field additions are backward-compatible; removals or type changes require a schema version bump.

### `overlay_units.customer_managed`

Controls generation of the `customer-managed/` overlay layer (a Flux `GitRepository` pointing at a *customer's own* repository, separate from the cluster's own GitOps repo above).

| Field | Type | Notes |
| --- | --- | --- |
| `enabled` | bool | Renders the whole `customer-managed/` tree when true. |
| `repository_name` | string | Logical name for the Flux `GitRepository`. |
| `repository_url` | string | Must be `ssh://` (required when `emit_secret` is true) or `https://`; `http://` is rejected. |
| `branch` | string | Branch to track. |
| `interval` | string | Reconciliation interval. |
| `flux_name_prefix` | string | Prefix applied to generated Flux object names. |
| `secret_name` | string | Name of the emitted Secret (only meaningful when `emit_secret` is true). |
| `emit_secret` | bool | When true, renders a Secret manifest carrying SSH credentials from `secrets.overlay_units.customer_managed` (see below); requires `known_hosts` to be non-empty and SSH transport. |
| `kustomizations` | list of `{name, path, depends_on}` | One entry per Flux Kustomization to generate against this repository. |

### `overlay_units.sops`

Controls generation of an overlay-local `.sops.yaml`.

| Field | Type | Notes |
| --- | --- | --- |
| `enabled` | bool | Renders `.sops.yaml` when true. |
| `rules` | list of `{path_regex, age_recipients, encrypted_regex}` | At least one rule is required when enabled; each rule needs a non-empty `path_regex` and at least one `age_recipient`. |

## `secrets.overlay_units.customer_managed` (`internal/config/overlay/types.go`, `Secrets`)

The secret-bearing counterpart to `opencenter.gitops.overlay_units.customer_managed`, kept in the separate `secrets.*` tree (which is where SOPS encryption rules typically target):

| Field | Type | Notes |
| --- | --- | --- |
| `identity` | string | SSH private key content (base64-encoded in the emitted Secret). |
| `identity_pub` | string | SSH public key content. |
| `known_hosts` | string | SSH known-hosts entry; required when `emit_secret` is true. Not content-validated beyond non-emptiness -- obtain it through a trusted channel (e.g. `ssh-keyscan` against a verified host). |

See [Overlay Rendering Security Policy](../contributing/overlay-security-policy.md) for the trust and validation rules around these fields, and [Renderer Contract](../contributing/rendering-contract.md) for how `customer-managed/` fits into renderer-owned vs. bootstrap-owned paths.

## Validation

All of the above is checked by [Validation Rules](validation-rules.md) (GitOps readiness checks) and by `internal/gitops/overlay_units_validation.go` at render time (transport scheme, `known_hosts` presence, SOPS rule completeness). See also [Cluster Validate Execution Flow](../contributing/validation.md).
