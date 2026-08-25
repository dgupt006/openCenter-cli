---
id: opencenter-secrets-set
title: "Opencenter_Secrets_Set"
sidebar_label: Opencenter_Secrets_Set
description: Documentation for Opencenter_Secrets_Set.
doc_type: reference
audience: "platform engineers"
tags: [reference]
---
## opencenter secrets set

Create or update a secret

```
opencenter secrets set <name> [flags]
```

### Options

```
      --from-file string          Path to a file containing the secret
  -h, --help                      help for set
      --label stringArray         Additional Barbican labels in key=value form
      --payload-encoding string   Encoding of the payload (e.g. base64) (default "base64")
      --secret-type string        Type of the secret (e.g. opaque, passphrase) (default "opaque")
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
