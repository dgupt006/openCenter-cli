---
id: configuration-precedence
title: "Configuration Precedence"
sidebar_label: Configuration Precedence
description: The two distinct precedence systems in openCenter -- cluster-config field merging, and CLI-tool path resolution.
doc_type: reference
audience: "operators, developers"
tags: [configuration, precedence, cli, reference]
---
# Configuration Precedence

**Purpose:** For operators and developers, explains how openCenter resolves a value when it could come from more than one place. There are two independent precedence systems -- do not conflate them.

## 1. Cluster-config field precedence (`internal/config/flags/configuration_merger.go`)

When building or updating a cluster's configuration, individual field values can originate from more than one source. `DefaultConfigurationMerger` merges them in this order, lowest to highest precedence:

```
SourceDefault  (lowest)
SourceFile
SourceTemplate
SourceCLI      (highest -- CLI flags always win)
```

* **`SourceDefault`** -- built-in defaults (`internal/config/v2/defaults.go`'s `NewV2Default`/`NewV2FullTemplate` and friends).
* **`SourceFile`** -- values loaded from an existing `.<cluster>-config.yaml` (or an explicit `--config-file`).
* **`SourceTemplate`** -- values applied from a named template (`--type <provider>` at `cluster init`, or an explicit template selection).
* **`SourceCLI`** -- values supplied directly as CLI flags, including dotted override flags (e.g. `opencenter.infrastructure.compute.worker_count=5` on `cluster init`/`cluster set`). These always take precedence over anything from a file, template, or default.

This is the precedence you're using when you run, e.g., `opencenter cluster init my-cluster --type openstack opencenter.infrastructure.compute.worker_count=5`: the default v2 config is built, provider-specific (OpenStack) template defaults are layered on, and the explicit `worker_count=5` CLI override wins over both.

`InitService.applyOverrides` (see [Cluster Init Details](../contributing/cluster-init-details.md)) also tracks which values were set explicitly (a map of "was this key touched by the user") specifically so that later path-resolution and Git-auth-default logic never silently overwrites a user-supplied value.

## 2. CLI-tool path precedence (per-directory, `internal/config/cli_settings_helpers.go`)

This is a *completely separate* precedence system that resolves *where on disk* the CLI reads/writes things -- it has nothing to do with cluster config field values. For every directory role (clusters, GitOps, blueprints, cluster state, secrets, plugins, general state), the resolution order is the same three steps:

```
1. The role's OPENCENTER_<X>_DIR environment variable, if set
2. The matching paths.<x>Dir value in ~/.config/opencenter/config.yaml (the CLI settings file)
3. A computed default (usually <clustersDir>/<role>, or a platform-specific base for clustersDir/configDir/stateDir themselves)
```

See [File Locations](file-locations.md) for the exact default path and environment variable for every role, and [Environment Variables](environment-variables.md) for the full list of recognized variables.

No environment variable overrides a cluster-config *field* value directly -- environment variables in this system only ever change *where files live*, never what a loaded cluster config's fields contain. The one partial exception is `OPENCENTER_DEBUG`, which changes CLI *behavior* (verbose logging, and triggers `cluster validate`'s debug-config export) rather than a config field.

## Practical implications

* Changing `OPENCENTER_CONFIG_DIR` mid-project does not change any value inside an already-loaded cluster config file; it changes which `config.yaml`/`clusters/` tree the CLI looks at next.
* A dotted CLI override on `cluster init`/`cluster set` always beats whatever is in the file or template -- if a value looks wrong after running one of these commands, check the exact flags passed before suspecting a stale file.
* `cluster configure --guided` reuses the same `InitService` internals (`createDefaultConfig`, `applyOverrides`, `updateConfigPaths`) when no config exists yet, so the same `SourceDefault -> SourceFile -> SourceTemplate -> SourceCLI` precedence applies there too.
