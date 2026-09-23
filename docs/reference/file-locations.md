---
id: file-locations
title: "File Locations"
sidebar_label: File Locations
description: Every configuration, state, cache, and generated-file location openCenter uses, with exact resolution order, verified against internal/config/cli_settings_helpers.go and internal/config/persistence.
doc_type: reference
audience: "all users"
tags: [files, paths, locations, configuration]
---
# File Locations

**Purpose:** For all users, documents the file and directory locations openCenter uses for configuration, runtime state, secrets, and generated files, and the exact order each one is resolved in.

Every directory role below is resolved the same way (see [Configuration Precedence](configuration-precedence.md) section 2): an `OPENCENTER_<X>_DIR` environment variable, then a `paths.<x>Dir` value in the CLI settings file, then a computed default.

## CLI settings and top-level directories

| Role | Env var | CLI settings key | Default |
| --- | --- | --- | --- |
| Config dir | `OPENCENTER_CONFIG_DIR` | `paths.settingsDir` | macOS/Linux: `~/.config/opencenter`; Windows: `%APPDATA%\opencenter` (falls back to `%LOCALAPPDATA%`, then `%USERPROFILE%`, then `/tmp/opencenter`) |
| CLI settings file | -- | -- | `<config-dir>/config.yaml` |
| Clusters dir | `OPENCENTER_CLUSTERS_DIR` | `paths.clustersDir` | `<config-dir>/clusters` |
| GitOps dir | `OPENCENTER_GITOPS_DIR` | `paths.gitopsDir` | `<clusters-dir>/gitops` |
| Blueprints dir | `OPENCENTER_BLUEPRINTS_DIR` | `paths.blueprintsDir` | `<clusters-dir>/blueprints` |
| Cluster state dir | `OPENCENTER_CLUSTER_STATE_DIR` | `paths.clusterStateDir` | `<clusters-dir>/state` |
| Secrets dir | `OPENCENTER_SECRETS_DIR` | `paths.secretsDir` | `<clusters-dir>/secrets` |
| Plugins dir | `OPENCENTER_PLUGINS_DIR` | `paths.pluginsDir` | `<config-dir>/plugins` |
| State dir (general runtime state) | `OPENCENTER_STATE_DIR` | `paths.stateDir` | macOS/Linux: `$XDG_STATE_HOME` or `~/.local/state/opencenter`; Windows: `%LOCALAPPDATA%\opencenter\state` (falls back similarly) |

`ResolveClustersDir` (used at `cluster init` time) has one extra fallback step between the CLI-settings value and the computed default: if `OPENCENTER_CONFIG_DIR` is set but no `paths.clustersDir` is configured, it uses `<OPENCENTER_CONFIG_DIR>/clusters` before falling back to the platform default's `/clusters`.

## Per-cluster layout (org-based)

```
<clusters-dir>/<organization>/
├── .<cluster>-config.yaml         # v2 cluster configuration (dot-prefixed filename)
├── infrastructure/clusters/<cluster>/     # OpenTofu working directory
├── applications/overlays/<cluster>/       # rendered GitOps overlay
├── secrets/
│   ├── age/keys/<cluster>-key.txt          # SOPS Age private key
│   └── ssh/<cluster>-<env>-<region>        # cluster SSH keypair
└── .sops.yaml                              # SOPS creation rules for this org/cluster tree
```

See [Cluster Init Details](../contributing/cluster-init-details.md) for exactly which of these paths `cluster init` writes into the config (`opencenter.gitops.repository.local_dir`, `infrastructure.ssh.key_path`, `secrets.ssh_key.private`/`.public`, `secrets.sops_age_key_file`, `secrets.sops.age_key_file`).

## Runtime state and logs

```
<state-dir>/
├── locks/<resource>.lock                                   # distributed/local file locks (LockManager)
├── bootstrap/<org>/<cluster>/state.json                    # cluster deploy resume state
└── logs/bootstrap/<org>/<cluster>/bootstrap-<timestamp>.log  # full deploy command output
```

**Known inconsistency:** the `kind-cleanup` mise task removes a stale lock at `${HOME}/.config/opencenter/locks/${CLUSTER}.lock` (i.e. under the *config* dir), but `LockManager` (`internal/resilience/lock_manager.go`) actually creates lock files under `<state-dir>/locks/<resource>.lock` (the *state* dir, default `~/.local/state/opencenter/locks/`). If your config dir and state dir differ (e.g. you've overridden `OPENCENTER_STATE_DIR`), the mise task's cleanup step will not find the real lock file -- clear it manually from `<state-dir>/locks/` in that case.

## Plugins

```
<plugins-dir>/
├── opencenter-local                # the bundled local-development plugin binary
└── checksums.txt                   # sha256 checksums registered by `mise run local-install`
```

External plugins are discovered as `opencenter-<name>` executables on `PATH` or in the plugins dir; see `internal/plugins/loader.go`.

## SOPS / Age keys outside the cluster tree

* `SOPS_AGE_KEY_FILE` (standard SOPS/Age environment variable, not an `OPENCENTER_*` variable) can point at a key file outside the per-cluster `secrets/age/` layout.
* The conventional per-user Age key directory is `~/.config/sops/age` (SOPS's own default, not an openCenter-specific path).

## Testing/CI-only paths

* `testdata/` at the repository root -- test fixtures, and (transiently) `testdata/config` when `OPENCENTER_CONFIG_DIR=./testdata/config` is used for isolated local test runs (see `.mise.toml`'s commented-out `OPENCENTER_CONFIG_DIR` default and `mise run schema-verify`). `mise run clean` deletes this directory -- do not store anything you want to keep there.
* CI (`deploy-kind.yml`) builds an entirely isolated set of `OPENCENTER_*_DIR` values under `${RUNNER_TEMP}` per run -- see [GitHub Actions Workflows](github-actions-workflows.md).

## Related references

* [Configuration Precedence](configuration-precedence.md) -- the resolution order these paths follow.
* [Environment Variables](environment-variables.md) -- the full variable list, including non-path variables.
* [Configuration Schema Reference](configuration-schema.md) -- what's actually stored in `.<cluster>-config.yaml`.
