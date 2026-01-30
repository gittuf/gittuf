## gittuf tui

Start the TUI for gittuf

### Synopsis

This command starts a terminal-based interface to view and/or manage gittuf metadata. If a signing key is provided, mutating operations are enabled and signed. Without a signing key, the TUI runs in read-only mode. Changes to the policy files in the TUI are staged immediately without further confirmation and users are required to run `gittuf policy apply` to commit the changes.

```
gittuf tui [flags]
```

### Options

```
      --create-rsl-entry     create RSL entry for policy change immediately (note: the RSL will not be synced with the remote)
  -h, --help                 help for tui
      --policy-name string   name of policy file to make changes to (default "targets")
      --read-only            interact with the TUI in read-only mode
  -k, --signing-key string   signing key to use to sign root of trust (path to SSH key, "fulcio:" for Sigstore)
      --target-ref string    specify which policy ref should be inspected (default "policy")
```

### Options inherited from parent commands

```
      --no-color                     turn off colored output
      --profile                      enable CPU and memory profiling
      --profile-CPU-file string      file to store CPU profile (default "cpu.prof")
      --profile-memory-file string   file to store memory profile (default "memory.prof")
      --verbose                      enable verbose logging
```

### SEE ALSO

* [gittuf](gittuf.md)	 - A security layer for Git repositories, powered by TUF

