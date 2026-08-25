---
id: opencenter-secrets-get
title: "Opencenter_Secrets_Get"
sidebar_label: Opencenter_Secrets_Get
description: Documentation for Opencenter_Secrets_Get.
doc_type: reference
audience: "platform engineers"
tags: [reference]
---
## opencenter secrets get

Download and decrypt a secret

```
opencenter secrets get <name> [flags]
```

### Options

```
  -h, --help                 help for get
      --output-file string   Path to save the secret
      --show                 Print secret to stdout (insecure)
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
