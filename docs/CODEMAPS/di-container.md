---
id: di-container-map
title: "Explain the Application Dependency Graph"
sidebar_label: DI Container
description: Explains how the openCenter CLI constructs its typed application graph, adapts it for commands, and separates canonical wiring from legacy compatibility.
doc_type: explanation
audience: "contributors, maintainers"
tags: [dependency-injection, runtime, services, security, wiring]
---
# Dependency injection and runtime wiring

The canonical application graph is the explicit constructor chain in `internal/di/app.go:NewApp`. It returns a typed `di.App`; `di.NewAppContainer` provides the compatibility `Container` interface used by older command code.

## Runtime wiring

```text
main.go
  -> resolve cluster base directory
  -> di.SetupContainer (outer process container/context bridge)
  -> cmd.ExecuteWithContext
       -> di.NewApp(baseDir)
       -> di.NewAppContainer(app)
       -> context{AppKey, ContainerKey}
       -> NewBuiltinRootCmd
       -> production-only LoadExternalPlugins
       -> Cobra ExecuteContext
```

The second executable, `cmd/opencenter-local/main.go`, has its own small Cobra root and does not use the production app graph; it constructs local-development services directly.

## Typed graph

`NewApp` constructs, in dependency order:

1. Error handler and filesystem.
2. `PathResolver` and logger.
3. Configuration manager and shared `ValidationEngine`.
4. Error formatting and security components: audit logger, input validator, credential masker, command sanitizer, and command runner.
5. Lifecycle services: `InitService`, `ConfigureService`, `ValidateService`, `SetupService`, and `BootstrapService`.

The resulting `di.App` exposes these components as typed fields. Commands retrieve it with `cmd.GetApp(ctx)` and use `cmd.GetContainer(ctx)` only where the compatibility interface is still required.

## Ownership table

| Component | Provider | Consumer boundary |
|---|---|---|
| Filesystem and paths | `internal/util/fs`, `internal/core/paths` | Config, lifecycle, GitOps, secrets, operations |
| Config manager | `internal/config` / `internal/config/v2` | Lifecycle and command helpers |
| Validation engine | `internal/core/validation` | Config and lifecycle readiness |
| Security services | `internal/security` | Commands and external-process boundaries |
| Lifecycle services | `internal/cluster` | Cobra command handlers |
| GitOps renderer | `internal/gitops` | `SetupService` and rendering tests |

## Legacy container boundary

`internal/di` still contains the reflection-based `Container` and `SetupContainer` implementation for compatibility and process shutdown. It is not the source of truth for the command-critical graph. Do not add new service wiring to the legacy registry when the typed `App` can express the dependency.

## Shutdown

The outer process container is shut down by `main.go` after command execution or before exiting on a command error. Components that own external resources must preserve the container's shutdown semantics; command code should return errors rather than terminate the process itself.

## Related maps

- [CLI commands](cli-commands.md) — command registration and context use
- [Config system](config-system.md) — configuration dependencies
- [Cluster lifecycle](cluster-lifecycle.md) — graph consumers
- [Runtime extensions and local development](runtime-extensions-and-local-development.md) — separate local executable and plugin boundary
