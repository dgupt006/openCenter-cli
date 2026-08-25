---
id: opencenter-cluster-drift-reconcile
title: "Opencenter_Cluster_Drift_Reconcile"
sidebar_label: Opencenter_Cluster_Drift_Reconcile
description: Documentation for Opencenter_Cluster_Drift_Reconcile.
doc_type: reference
audience: "platform engineers"
tags: [reference]
---
## opencenter cluster drift reconcile

Reconcile detected infrastructure drift

### Synopsis

Reconcile differences between desired configuration and actual infrastructure state.

This command first detects drift, then applies changes to bring the actual infrastructure
state back in line with the desired configuration. Only reconcilable drift can be fixed
automatically. Non-reconcilable drift (e.g., deleted resources, manual resource creation)
requires manual intervention.

Use --dry-run to see what changes would be made without applying them.

If no cluster name is provided, uses the currently active cluster.

```
opencenter cluster drift reconcile [cluster] [flags]
```

### Examples

```
  # Reconcile drift for active cluster
  opencenter cluster drift reconcile

  # Show what would be reconciled (dry-run)
  opencenter cluster drift reconcile my-cluster --dry-run

  # Apply reconciliation
  opencenter cluster drift reconcile my-cluster

  # Reconcile with confirmation prompt
  opencenter cluster drift reconcile my-cluster --confirm
```

### Options

```
      --confirm      Prompt for confirmation before applying changes
      --dry-run      Show what would be changed without applying
  -h, --help         help for reconcile
      --to-cluster   Reconcile infrastructure to match the current config (default)
      --to-config    Promote approved live cluster state back into config
```

### Options inherited from parent commands

```
      --config-dir string   configuration directory (defaults to ~/.config/opencenter on Linux/macOS)
      --log-level string    set log level explicitly (debug, info, warn, error) (default "warn")
      --output string       output format for supported commands: text, json, yaml (default "text")
      --quiet               suppress nonessential human output
      --yes                 answer yes to confirmation prompts
```

### SEE ALSO

* [opencenter cluster drift](opencenter_cluster_drift.md)	 - Detect and reconcile infrastructure drift
