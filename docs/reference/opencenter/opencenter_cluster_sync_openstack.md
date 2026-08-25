---
id: "opencenter-cluster-sync-openstack"
title: "Opencenter_Cluster_Sync_Openstack"
sidebar_label: "Opencenter_Cluster_Sync_Openstack"
description: "Synchronize a cluster with an OpenStack clouds.yaml profile"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
---
## opencenter cluster sync openstack

Synchronize a cluster with an OpenStack clouds.yaml profile

### Synopsis

Discover OpenStack values from a clouds.yaml profile and reconcile them into
the cluster configuration. Storage service wiring creates Swift containers and
credentials only when the configuration does not already contain them, unless
--rotate-creds is supplied.

```
opencenter cluster sync openstack <cluster> [flags]
```

### Examples

```
  # Preview discovery and configuration changes
  opencenter cluster sync openstack acme/prod --os-cloud production --dry-run

  # Reconcile core settings and selected storage services
  opencenter cluster sync openstack acme/prod --os-cloud production \
    --services loki=swift,tempo=s3 --yes
```

### Options

```
      --clouds-yaml string   path to clouds.yaml (defaults to OS_CLIENT_CONFIG_FILE or ~/.config/openstack/clouds.yaml)
  -h, --help                 help for openstack
      --match-flavors        select fitting OpenStack flavors for cluster roles
      --match-volume-type    select an available OpenStack volume type
      --no-scope-creds       allow unscoped Swift application credentials
      --os-cloud string      clouds.yaml profile to use (defaults to OS_CLOUD)
      --rotate-creds         create replacement service credentials
      --services string      comma-separated storage mappings, for example loki=swift,tempo=s3
      --subnet-id string     internal subnet ID to use when discovery finds multiple subnets
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

* [opencenter cluster sync](opencenter_cluster_sync.md)	 - Synchronize cluster configuration from external systems
