---
id: opencenter-secrets-delete
title: "Opencenter_Secrets_Delete"
sidebar_label: Opencenter_Secrets_Delete
description: Documentation for Opencenter_Secrets_Delete.
doc_type: reference
audience: "platform engineers"
tags: [reference]
---
## opencenter secrets delete

Delete a secret

```
opencenter secrets delete <name> [flags]
```

### Options

```
      --force   Force deletion of a secret
  -h, --help    help for delete
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

* [opencenter secrets](opencenter_secrets.md)	 - Manage secrets across backends
