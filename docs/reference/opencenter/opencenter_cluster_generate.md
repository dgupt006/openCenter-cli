---
id: opencenter-cluster-generate
title: "Opencenter_Cluster_Generate"
sidebar_label: Opencenter_Cluster_Generate
description: Documentation for Opencenter_Cluster_Generate.
doc_type: reference
audience: "platform engineers"
tags: [reference]
---
## opencenter cluster generate

Generate the GitOps repository and rendered manifests

### Synopsis

Generate the customer GitOps repository and rendered manifests for a cluster.

This command creates or updates the repository structure, infrastructure templates,
Flux manifests, and application overlays based on the cluster configuration.

Use --render-only to render templates without running the full repository setup flow.
Use --gitops-auth to select the GitOps authentication method for the base repository
sources (ssh or token). Defaults to cluster_defaults.gitops_auth_method from settings.

```
opencenter cluster generate [name] [flags]
```

### Examples

```
  # Generate assets for the active cluster
  opencenter cluster generate

  # Generate assets for a specific cluster
  opencenter cluster generate my-cluster

  # Preview what would be generated
  opencenter cluster generate my-cluster --dry-run

  # Render templates only
  opencenter cluster generate my-cluster --render-only

  # Generate with explicit GitOps auth method
  opencenter cluster generate my-cluster --gitops-auth=token
  opencenter cluster generate my-cluster --gitops-auth=ssh
```

### Options

```
      --force                overwrite existing GitOps repository
      --gitops-auth string   GitOps authentication method for base repo sources (ssh, token); defaults to cluster_defaults.gitops_auth_method
  -h, --help                 help for generate
      --render-only          render templates without running repository setup
      --skip-validation      skip configuration validation before generation
```

### Options inherited from parent commands

```
      --config-dir string   configuration directory (defaults to ~/.config/opencenter on Linux/macOS)
      --dry-run             preview mutating operations without writing or acting
      --log-level string    set log level explicitly (debug, info, warn, error) (default "warn")
      --output string       output format for supported commands: text, json, yaml (default "text")
      --quiet               suppress nonessential human output
      --yes                 answer yes to confirmation prompts
```

### SEE ALSO

* [opencenter cluster](opencenter_cluster.md)	 - Manage cluster configurations
