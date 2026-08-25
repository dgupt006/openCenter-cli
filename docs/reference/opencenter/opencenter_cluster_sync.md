---
id: "opencenter-cluster-sync"
title: "Opencenter_Cluster_Sync"
sidebar_label: "Opencenter_Cluster_Sync"
description: "Synchronize cluster configuration from external systems"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
---
## opencenter cluster sync

Synchronize cluster configuration from external systems

```
opencenter cluster sync [flags]
```

### Options

```
  -h, --help   help for sync
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
* [opencenter cluster sync openstack](opencenter_cluster_sync_openstack.md)	 - Synchronize a cluster with an OpenStack clouds.yaml profile
