## gittuf tui

Start the TUI for gittuf

### Synopsis

The 'tui' command starts a terminal-based interface to view and manage gittuf policy metadata. It is used to inspect or modify policy files interactively. A signing key must be provided to enable write operations; without one the TUI runs in read-only mode. Changes made in the TUI are staged immediately and require running 'gittuf policy apply' to take effect.

```
gittuf tui [flags]
```

### Options

```
      --create-rsl-entry     create RSL entry for policy change immediately (note: the RSL will not be synced with the remote)
  -h, --help                 help for tui
      --policy-name string   name of policy file to make changes to (default "targets")
      --read-only            interact with the TUI in read-only mode
  -k, --signing-key string   signing key to use to sign policy metadata (path to SSH key, "gpg:<fingerprint>" for GPG, "fulcio:" for Sigstore)
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

