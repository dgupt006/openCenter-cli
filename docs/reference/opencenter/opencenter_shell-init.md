---
id: opencenter-shell-init
title: "Opencenter_Shell Init"
sidebar_label: Opencenter_Shell Init
description: Documentation for Opencenter_Shell Init.
doc_type: reference
audience: "platform engineers"
tags: [reference]
---
## opencenter shell-init

Output shell integration script for session-scoped active clusters

### Synopsis

Outputs shell integration code to be evaluated in your shell.

This enables session-scoped active clusters, where each terminal can have
its own active cluster context without affecting other terminals.

Add to your shell configuration file:

  Bash (~/.bashrc):
    eval "$(opencenter shell-init)"

  Zsh (~/.zshrc):
    eval "$(opencenter shell-init)"

  Fish (~/.config/fish/config.fish):
    opencenter shell-init --shell fish | source

Features:
  • Session-isolated cluster contexts
  • Visual prompt indicator showing active cluster
  • Automatic cleanup on shell exit
  • Compatible with existing persistent selection

```
opencenter shell-init [flags]
```

### Examples

```
  # Auto-detect shell and output integration script
  opencenter shell-init

  # Specify shell explicitly
  opencenter shell-init --shell zsh

  # Install to shell config
  echo 'eval "$(opencenter shell-init)"' >> ~/.zshrc
```

### Options

```
  -h, --help           help for shell-init
      --shell string   Shell type (bash, zsh, fish) - auto-detected if not specified
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

* [opencenter](opencenter.md)	 - opencenter CLI manages cluster configurations and GitOps scaffolding
