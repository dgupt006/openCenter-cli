---
id: opencenter-secrets-login
title: "Opencenter_Secrets_Login"
sidebar_label: Opencenter_Secrets_Login
description: Documentation for Opencenter_Secrets_Login.
doc_type: reference
audience: "platform engineers"
tags: [reference]
---
## opencenter secrets login

Create or refresh a Keystone token

```
opencenter secrets login [flags]
```

### Options

```
  -h, --help                help for login
      --password-stdin      Read password from stdin (required for non-interactive use)
      --project-id string   OpenStack project ID
      --username string     OpenStack username
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
