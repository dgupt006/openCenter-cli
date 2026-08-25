---
id: opencenter-cluster-service-status
title: "Opencenter_Cluster_Service_Status"
sidebar_label: Opencenter_Cluster_Service_Status
description: Documentation for Opencenter_Cluster_Service_Status.
doc_type: reference
audience: "platform engineers"
tags: [reference]
---
## opencenter cluster service status

Display state of all services in the cluster configuration

### Synopsis

Display all services (standard and managed) with their enabled/disabled state
and adoption mode. For live deployment status, use 'opencenter cluster status --sync'.

Examples:
  # Show state of all services in the active cluster
  opencenter cluster service status

  # Show state for a specific cluster
  opencenter cluster service status --cluster my-cluster

```
opencenter cluster service status [flags]
```

### Options

```
      --cluster string   Specify the cluster name
  -h, --help             help for status
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

* [opencenter cluster service](opencenter_cluster_service.md)	 - Manage cluster services
