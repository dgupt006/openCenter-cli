---
id: runtime-extensions-and-local-development
title: "Explain Runtime Extensions and Local Development"
sidebar_label: Runtime Extensions
description: Explains production external-plugin discovery, the separate opencenter-local executable, reusable template services, and security controls around extension processes.
doc_type: explanation
audience: "contributors, maintainers, plugin authors"
tags: [plugins, local-development, gitea, flux, templates]
---
# Runtime extensions and local development

Runtime extensibility and local development are separate from the built-in production command graph. External plugins extend the production executable at runtime; `opencenter-local` is a second executable that orchestrates disposable local infrastructure.

## External plugin flow

```text
cmd.NewBuiltinRootCmd
  -> production ExecuteWithContext
  -> internal/plugins.LoadExternalPlugins
       OPENCENTER_PLUGINS_DIR
       <config-dir>/plugins
       PATH
  -> executable names prefixed opencenter-
  -> checksum status
  -> attach non-conflicting Cobra command
  -> forward arguments through internal/security.CommandRunner
```

Plugins with verified checksums run normally; unverified plugins warn; checksum mismatches and verification errors are refused. Built-in command names cannot be shadowed. External plugins are discovered only in the production runtime path. The generated Cobra reference calls `NewBuiltinRootCmd()` directly and therefore does not load external plugins.

## Local executable

`cmd/opencenter-local/main.go` creates a separate Cobra root with:

| Command | Implementation | Role |
|---|---|---|
| `gitea up`, `status`, `destroy`, `attach-kind` | `internal/localdev/gitea` | Manage disposable local Gitea, credentials, repository, and Kind network attachment |
| `gitops push` | `internal/localdev/gitops` | Operate on the local GitOps repository for a resolved cluster |
| `flux bootstrap` | `internal/localdev/flux` | Bootstrap Flux from local Gitea |

`internal/localdev.ClusterResolver` loads a validated cluster config through the shared configuration manager and resolves organization-aware paths. `localdev.Executor` is the command execution boundary used by local services.

## Template boundary

`internal/template` provides a reusable Go `TemplateEngine` with string/file rendering, syntax validation, function registration, caching, composition, embedded registries, and sandboxing. `internal/gitops` owns which embedded templates are used for cluster generation and how their outputs are owned; the generic template package does not choose provider or service topology.

Generated repository templates are embedded in the binary. They do not load external plugins. External plugins are executable Cobra extensions, not template sources.

## Security boundary

`internal/security` is shared by production and local command paths:

- `CommandSanitizer` and `CommandRunner` construct external commands safely;
- `InputValidator` validates user-controlled identifiers and values;
- `CredentialMasker` removes sensitive values from logs and diagnostics; and
- `AuditLogger` emits HMAC-protected security events when enabled.

Extension and local-development code should pass commands through these boundaries rather than calling `os/exec` directly or duplicating credential handling.

## Related maps

- [CLI commands](cli-commands.md) — production registration and plugin timing
- [DI container](di-container.md) — canonical production graph
- [GitOps engine](gitops-engine.md) — embedded generation templates
- [Providers](providers.md) — Kind and local-development relationship
