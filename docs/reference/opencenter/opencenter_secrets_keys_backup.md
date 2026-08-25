---
id: opencenter-secrets-keys-backup
title: "Opencenter_Secrets_Keys_Backup"
sidebar_label: Opencenter_Secrets_Keys_Backup
description: Documentation for Opencenter_Secrets_Keys_Backup.
doc_type: reference
audience: "platform engineers"
tags: [reference]
---
## opencenter secrets keys backup

Create backup of Age keys and SOPS configuration

### Synopsis

Create a secure backup of Age keys and SOPS configuration.

This command creates a timestamped backup of the Age key file and .sops.yaml
configuration. Backups are essential for disaster recovery and should be stored
securely in a separate location from the primary keys.

The backup includes:
• Age private key file
• SOPS configuration (.sops.yaml)
• Backup metadata with timestamp and creation details

```
opencenter secrets keys backup [flags]
```

### Options

```
      --backup-dir string   Backup directory (default: ~/.config/sops/age/backups)
      --dry-run             Show what would be done without making changes
  -h, --help                help for backup
      --key-file string     Path to Age key file (default: ~/.config/sops/age/keys.txt)
```

### Options inherited from parent commands

```
      --config-dir string   configuration directory (defaults to ~/.config/opencenter on Linux/macOS)
      --log-level string    set log level explicitly (debug, info, warn, error) (default "warn")
      --output string       output format for supported commands: text, json, yaml (default "text")
      --quiet               suppress nonessential human output
      --yes                 answer yes to confirmation prompts
```

### SEE ALSO

* [opencenter secrets keys](opencenter_secrets_keys.md)	 - Manage SOPS encryption keys
