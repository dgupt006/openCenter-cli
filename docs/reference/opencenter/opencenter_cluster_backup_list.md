---
id: opencenter-cluster-backup-list
title: "Opencenter_Cluster_Backup_List"
sidebar_label: Opencenter_Cluster_Backup_List
description: Documentation for Opencenter_Cluster_Backup_List.
doc_type: reference
audience: "platform engineers"
tags: [reference]
---
## opencenter cluster backup list

List backups for a cluster

### Synopsis

List all backups for a cluster or all backups if no cluster is specified.

Displays backup ID, creation time, size, and storage location.

```
opencenter cluster backup list [cluster] [flags]
```

### Examples

```
  # List all backups
  opencenter cluster backup list

  # List backups for a specific cluster
  opencenter cluster backup list my-cluster
```

### Options

```
  -h, --help   help for list
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

* [opencenter cluster backup](opencenter_cluster_backup.md)	 - Manage cluster backups
