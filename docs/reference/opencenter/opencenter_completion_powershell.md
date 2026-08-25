---
id: opencenter-completion-powershell
title: "Opencenter_Completion_Powershell"
sidebar_label: Opencenter_Completion_Powershell
description: Documentation for Opencenter_Completion_Powershell.
doc_type: reference
audience: "platform engineers"
tags: [reference]
---
## opencenter completion powershell

Generate the autocompletion script for powershell

### Synopsis

Generate the autocompletion script for powershell.

To load completions in your current shell session:

	opencenter completion powershell | Out-String | Invoke-Expression

To load completions for every new session, add the output of the above command
to your powershell profile.


```
opencenter completion powershell [flags]
```

### Options

```
  -h, --help              help for powershell
      --no-descriptions   disable completion descriptions
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

* [opencenter completion](opencenter_completion.md)	 - Generate the autocompletion script for the specified shell
