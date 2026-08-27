---
id: "opencenter-cluster-provider-openstack-plan"
title: "Opencenter_Cluster_Provider_Openstack_Plan"
sidebar_label: "Opencenter_Cluster_Provider_Openstack_Plan"
description: "plan OpenStack provider metadata and selections"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
---
## opencenter cluster provider openstack plan

plan OpenStack provider metadata and selections

```
opencenter cluster provider openstack plan <cluster> [flags]
```

### Options

```
      --availability-zone string     availability zone
      --clouds-yaml string           path to clouds.yaml
      --external-network-id string   external network ID
  -h, --help                         help for plan
      --image-id string              Linux image ID
      --import-auth                  write application credential fields from the selected profile
      --import-tls                   persist selected profile TLS settings
      --network-id string            internal network ID
      --os-cloud string              clouds.yaml profile to use
      --replace                      allow replacing populated provider selections
      --subnet-id string             internal subnet ID
      --windows-image-id string      Windows image ID
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

* [opencenter cluster provider openstack](opencenter_cluster_provider_openstack.md)	 - Manage OpenStack provider configuration
