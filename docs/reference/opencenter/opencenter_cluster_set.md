---
id: opencenter-cluster-set
title: "Opencenter_Cluster_Set"
sidebar_label: Opencenter_Cluster_Set
description: Documentation for Opencenter_Cluster_Set.
doc_type: reference
audience: "platform engineers"
tags: [reference]
---
## opencenter cluster set

Set fields in an existing cluster configuration

### Synopsis

Set one or more fields in an existing cluster configuration.

Fields use native v2 dot notation, for example:
  opencenter.meta.env=prod
  opencenter.gitops.repository.url=https://github.com/acme/platform.git

```
opencenter cluster set [cluster] <path=value>... [flags]
```

### Options

```
  -h, --help     help for set
      --strict   fail if the resulting configuration is not valid
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
