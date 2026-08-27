---
id: cli-commands-map
title: "Map the Built-in CLI Commands"
sidebar_label: CLI Commands
description: Describes the deterministic built-in Cobra tree, command registration ownership, and the production-only external plugin boundary.
doc_type: explanation
audience: "contributors, maintainers, CLI integrators"
tags: [cli, cobra, commands, plugins, runtime]
---
# CLI commands

The production executable starts with `cmd.NewBuiltinRootCmd()` and adds external plugins afterward. The tools-tagged reference generator also receives `NewBuiltinRootCmd()`, so generated command pages cover built-ins only; external plugins are discovered only at production runtime.

## Built-in tree

```text
opencenter
├── cluster
│   ├── list, use, active, env, status, describe
│   ├── init, configure, edit, set, normalize, export
│   ├── validate, doctor, generate, deploy, destroy
│   ├── template (hidden), validate-manifests (hidden)
│   ├── service {enable, disable, status, options}
│   ├── pool {add, update, scale, remove, list}
│   ├── drift {detect, reconcile, schedule}
│   ├── backup {create, restore, list, delete, schedule}
│   ├── lock, unlock
│   ├── import {scan, report, apply}
│   ├── migrate-layout
│   └── sync {openstack}
├── settings {view, set, get, reset, path, edit, explain, ide}
├── secrets
│   ├── login, list, describe, get, set, delete
│   ├── sync, validate, encrypt, decrypt, status
│   └── keys {generate, rotate, backup, validate, check, revoke, reconcile, set-primary}
├── plugins {list}
├── version
└── shell-init
```

Cobra also supplies the standard `help` and `completion` commands. Hidden commands are registered for internal workflows but are not part of the normal visible surface. Key lifecycle operations live under `secrets keys`; older cluster-level aliases are not part of the registered tree.

## Registration flow

| Stage | Owner | Responsibility |
|---|---|---|
| Process start | `main.go` | Set build metadata, resolve cluster directory, create the outer process container |
| App execution | `cmd/root.go:ExecuteWithContext` | Build `di.NewApp`, create `di.NewAppContainer`, and place both graph values in context |
| Built-ins | `cmd/root.go:NewBuiltinRootCmd` | Register `cluster`, `settings`, `secrets`, `plugins`, `version`, and `shell-init` |
| Cluster subtree | `cmd/cluster.go:NewClusterCmd` | Register lifecycle, service, pool, drift, backup, import, lock, and sync commands |
| Runtime extensions | `internal/plugins/loader.go` | Discover and attach external `opencenter-*` executables only in production |
| Execution | Cobra `ExecuteContext` | Parse flags, run command hooks, resolve services, and return errors to `main.go` |

Command implementations remain in `cmd/`; domain behavior belongs in `internal/*`. Commands should resolve typed services through the app graph rather than duplicate config, rendering, or provider logic.

## Global flags

`cmd/root.go` owns the persistent built-in flags:

| Flag | Purpose |
|---|---|
| `--config-dir` | Override the configuration directory; it is pre-parsed so runtime plugin discovery sees it |
| `--log-level` | Set logging level |
| `--output` | Select supported text, JSON, or YAML output where the command exposes it |
| `--quiet` | Suppress nonessential human output |
| `--yes` | Answer confirmations |
| `--dry-run` | Preview supported mutating operations |

## Boundaries

- `cluster generate` delegates to `internal/cluster.SetupService`; it does not call the supporting `PipelineGenerator` as the live top-level path.
- `secrets sync` delegates manifest work to `internal/secrets`; SOPS encryption and key operations are separate concerns.
- `plugins list` reports discovery; executing a plugin forwards arguments through the security command runner.
- `cmd/opencenter-local` is a separate executable, not a subcommand of the built-in production tree. See [Runtime extensions and local development](runtime-extensions-and-local-development.md).

## Related maps

- [DI container](di-container.md) — graph construction and command context
- [Cluster lifecycle](cluster-lifecycle.md) — command-to-service workflow
- [Secrets management](secrets-management.md) — secret command boundaries
- [Runtime extensions and local development](runtime-extensions-and-local-development.md) — plugin discovery and local executable
