---
id: environment-variables
title: "Environment Variables"
sidebar_label: Environment Variables
description: Every environment variable openCenter CLI reads, verified by source grep, with what it controls and its default.
doc_type: reference
audience: "all users"
tags: [environment, variables, configuration, cli]
---
# Environment Variables

**Purpose:** For all users, documents every environment variable openCenter CLI recognizes. Every variable below was confirmed with a source grep (`os.Getenv`/`os.LookupEnv`) against `internal/` and `cmd/`; none are invented.

## Path-resolution variables

These only change *where files live* -- see [Configuration Precedence](configuration-precedence.md) and [File Locations](file-locations.md) for the full resolution order (env var, then CLI settings file, then computed default).

| Variable | Controls | Default if unset |
| --- | --- | --- |
| `OPENCENTER_CONFIG_DIR` | CLI settings/config directory | `~/.config/opencenter` (or platform equivalent) |
| `OPENCENTER_CLUSTERS_DIR` | Cluster storage root | `<config-dir>/clusters` |
| `OPENCENTER_GITOPS_DIR` | GitOps repository root | `<clusters-dir>/gitops` |
| `OPENCENTER_BLUEPRINTS_DIR` | Cluster blueprints root | `<clusters-dir>/blueprints` |
| `OPENCENTER_CLUSTER_STATE_DIR` | Per-cluster state root | `<clusters-dir>/state` |
| `OPENCENTER_SECRETS_DIR` | Per-cluster secrets root | `<clusters-dir>/secrets` |
| `OPENCENTER_PLUGINS_DIR` | External plugin discovery/install directory | `<config-dir>/plugins` |
| `OPENCENTER_STATE_DIR` | General runtime state (locks, bootstrap state, logs) | `~/.local/state/opencenter` (or `$XDG_STATE_HOME`, or platform equivalent) |

## Behavior variables

| Variable | Controls | Notes |
| --- | --- | --- |
| `OPENCENTER_CLUSTER` | Active-cluster override | Read by `internal/config/v2/manager.go` and `cmd/cluster_active.go` as one of the sources for "which cluster am I operating on" resolution (alongside the persistent active-cluster marker and `cluster use` session state). |
| `OPENCENTER_SESSION_FILE` | Active-cluster session file path | Used by `cluster use`/`cluster active` to persist/read the shell-session-scoped active cluster, as an alternative to the global active-cluster marker. |
| `OPENCENTER_DEBUG` | Enables debug behavior | When set (to any non-empty value), `cluster validate` exports the effective resolved config to `.opencenter-v2.yaml` (same as passing `--generate-debug-config`); `secrets keys` commands print extra environment diagnostics. |
| `OPENCENTER_LOG_LEVEL` | Default log level | One of `debug`, `info`, `warn`, `error`. Only takes effect if `--log-level` was not explicitly passed on the command line. Default (when neither the flag nor this variable is set) is `warn`. |
| `OPENCENTER_TEST_MODE` | Skips real side effects in specific code paths | Confirmed at least in the Kind bootstrap provider's `gitea-attach-kind` step, where it short-circuits the real Gitea-service call entirely. Intended for tests, not documented as a general user-facing switch -- do not set it in normal use. |
| `OPENCENTER_ALLOW_INSECURE_FILE_MODES` | Relaxes a file-permission check | Set to exactly `1` to bypass a strict file-mode check in `internal/cluster/init_service.go`. Security-relevant -- only use for environments (e.g. certain CI filesystems) where the normal permission model genuinely cannot apply. |
| `OPENCENTER_GUIDED_ANSWERS` | Pre-scripted answers for guided flows | Read by `internal/ui/guided_prompter.go` (`guidedAnswersEnv`) to drive `cluster configure --guided` non-interactively -- point it at a file of pre-supplied answers instead of prompting. |

## Non-`OPENCENTER_*` variables the CLI reads

| Variable | Used for |
| --- | --- |
| `SOPS_AGE_KEY_FILE` | Standard SOPS/Age variable -- path to an Age private key file, consulted by SOPS operations outside the per-cluster `secrets/age/` layout. |
| `KUBECONFIG` | Standard kubeconfig path; set explicitly by `cluster deploy`/bootstrap steps rather than relying on ambient state. |
| `EDITOR`, `VISUAL` | Used by `cluster edit`/`config edit`/`settings edit` to launch an interactive editor. |
| `HOME`, `SHELL`, `PATH` | Standard shell environment, used for home-directory resolution and shell-integration setup (`hack/install-shell-integration.sh`). |
| `XDG_STATE_HOME` | XDG state-directory base on Linux/macOS, consulted by `DefaultStateDir()`. |
| `USER`, `USERNAME` | Used where a default "created by" / audit identity is needed. |
| `APPDATA`, `LOCALAPPDATA`, `USERPROFILE` | Windows path resolution fallbacks for the config/state directory defaults. |
| `CONTAINER_RUNTIME` | Selects `docker` or `podman` for local Kind/Gitea workflows (set via `.mise.toml`'s `[env]` block, or override on the command line). |
| `KIND_EXPERIMENTAL_PROVIDER` | Tells the `kind` binary to use Podman instead of Docker (set via `.mise.toml`). |
| `OS_CLIENT_CONFIG_FILE`, `OS_USERNAME`, `OS_PASSWORD`, and the rest of the standard `OS_*` OpenStack client variables | Read when extracting OpenStack credentials for bootstrap/preflight -- see [`cluster deploy` -- OpenStack Provider](../contributing/cluster-deploy-openstack.md) for the full `OS_*` variable table injected into bootstrap steps. |

## Cross-references

* [Configuration Precedence](configuration-precedence.md) -- how these interact with the CLI settings file and computed defaults.
* [File Locations](file-locations.md) -- the exact directory each path variable controls.
* [Exit Codes](exit-codes.md) -- process exit-code contract (not environment-variable controlled).
