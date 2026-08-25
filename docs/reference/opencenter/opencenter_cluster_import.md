---
id: opencenter-cluster-import
title: "Opencenter_Cluster_Import"
sidebar_label: Opencenter_Cluster_Import
description: Documentation for Opencenter_Cluster_Import.
doc_type: reference
audience: "platform engineers"
tags: [reference]
---
## opencenter cluster import

Import running clusters into openCenter config

### Synopsis

Discover cluster metadata from GitOps sources with kubeconfig fallback,
persist the discovery artifact, and create or patch openCenter cluster configs.

```
opencenter cluster import [flags]
```

### Options

```
  -h, --help   help for import
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
* [opencenter cluster import apply](opencenter_cluster_import_apply.md)	 - Create or patch openCenter configs from the latest import artifact
* [opencenter cluster import report](opencenter_cluster_import_report.md)	 - Render the latest cluster import artifact for a repo
* [opencenter cluster import scan](opencenter_cluster_import_scan.md)	 - Scan a customer GitOps repo and persist a cluster import artifact
