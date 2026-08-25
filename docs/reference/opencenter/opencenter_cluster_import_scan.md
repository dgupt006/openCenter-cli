---
id: opencenter-cluster-import-scan
title: "Opencenter_Cluster_Import_Scan"
sidebar_label: Opencenter_Cluster_Import_Scan
description: Documentation for Opencenter_Cluster_Import_Scan.
doc_type: reference
audience: "platform engineers"
tags: [reference]
---
## opencenter cluster import scan

Scan a customer GitOps repo and persist a cluster import artifact

```
opencenter cluster import scan [flags]
```

### Options

```
  -h, --help                            help for scan
      --repo string                     Path to the customer GitOps repository
      --service-namespace stringArray   Override service namespace ownership (svc=ns1,ns2)
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

* [opencenter cluster import](opencenter_cluster_import.md)	 - Import running clusters into openCenter config
