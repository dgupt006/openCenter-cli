---
id: opencenter-cluster-drift-schedule
title: "Opencenter_Cluster_Drift_Schedule"
sidebar_label: Opencenter_Cluster_Drift_Schedule
description: Documentation for Opencenter_Cluster_Drift_Schedule.
doc_type: reference
audience: "platform engineers"
tags: [reference]
---
## opencenter cluster drift schedule

Schedule periodic drift detection

### Synopsis

Schedule periodic drift detection for a cluster.

This command sets up a background process that periodically checks for drift and
reports results. Drift reports can be sent to a callback URL or logged locally.

If no cluster name is provided, uses the currently active cluster.

```
opencenter cluster drift schedule [cluster] [flags]
```

### Examples

```
  # Schedule drift detection for active cluster every 24 hours
  opencenter cluster drift schedule --interval=24h

  # Schedule for specific cluster every 24 hours
  opencenter cluster drift schedule my-cluster --interval=24h

  # Schedule with custom callback
  opencenter cluster drift schedule my-cluster --interval=12h --callback=https://example.com/drift
```

### Options

```
      --callback string   Callback URL for drift reports
  -h, --help              help for schedule
      --interval string   Interval between drift checks (e.g., 1h, 24h, 7d) (default "24h")
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

* [opencenter cluster drift](opencenter_cluster_drift.md)	 - Detect and reconcile infrastructure drift
