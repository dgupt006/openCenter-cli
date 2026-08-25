---
id: opencenter-cluster-configure
title: "Opencenter_Cluster_Configure"
sidebar_label: Opencenter_Cluster_Configure
description: Documentation for Opencenter_Cluster_Configure.
doc_type: reference
audience: "platform engineers"
tags: [reference]
---
## opencenter cluster configure

Guided cluster configuration for supported providers

```
opencenter cluster configure [name] [flags]
```

### Options

```
  -h, --help          help for configure
      --org string    organization for new cluster configurations
      --type string   infrastructure provider for new cluster configurations (default "openstack")
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
