## gittuf policy tui

Start the TUI for managing policies

### Synopsis

This command allows users to start a terminal-based interface to manage policies. The signing key specified will be used to sign all operations while in the TUI. Changes to the policy files in the TUI are staged immediately without further confirmation and users are required to run `gittuf policy apply` to commit the changes

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

