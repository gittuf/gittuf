## gittuf policy tui

Start the TUI for managing policies

### Synopsis

This command starts a terminal-based interface to view and/or manage gittuf metadata. If a signing key is provided, mutating operations are enabled and signed. Without a signing key, the TUI runs in read-only mode.

The TUI supports managing:
- Policy rules
- Trust global rules
- Trust root principals and keys

Changes are staged immediately without further confirmation. Run `gittuf policy apply` to apply policy rule changes, and run `gittuf trust apply` to apply trust metadata changes.

```
gittuf policy tui [flags]
```

### Options

```
  -h, --help                 help for tui
      --policy-name string   name of policy file to make changes to (default "targets")
      --target-ref string    specify which policy ref should be inspected (default "policy")
```

### Options inherited from parent commands

```
      --create-rsl-entry             create RSL entry for policy change immediately (note: the RSL will not be synced with the remote)
      --no-color                     turn off colored output
      --profile                      enable CPU and memory profiling
      --profile-CPU-file string      file to store CPU profile (default "cpu.prof")
      --profile-memory-file string   file to store memory profile (default "memory.prof")
  -k, --signing-key string           signing key to use to sign root of trust (path to SSH key, "fulcio:" for Sigstore)
      --verbose                      enable verbose logging
```

### SEE ALSO

* [gittuf policy](gittuf_policy.md)	 - Tools to manage gittuf policies

