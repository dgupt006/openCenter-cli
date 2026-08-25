---
id: opencenter-cluster-import-report
title: "Opencenter_Cluster_Import_Report"
sidebar_label: Opencenter_Cluster_Import_Report
description: Documentation for Opencenter_Cluster_Import_Report.
doc_type: reference
audience: "platform engineers"
tags: [reference]
---
## opencenter cluster import report

Render the latest cluster import artifact for a repo

```
opencenter cluster import report [flags]
```

### Options

```
  -h, --help            help for report
      --output string   Output format (text, json, yaml) (default "text")
      --repo string     Path to the customer GitOps repository
```

### Options inherited from parent commands

```
      --config-dir string   configuration directory (defaults to ~/.config/opencenter on Linux/macOS)
      --dry-run             preview mutating operations without writing or acting
      --log-level string    set log level explicitly (debug, info, warn, error) (default "warn")
      --quiet               suppress nonessential human output
      --yes                 answer yes to confirmation prompts
```

### SEE ALSO

* [opencenter cluster import](opencenter_cluster_import.md)	 - Import running clusters into openCenter config
