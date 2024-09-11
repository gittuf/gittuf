## gittuf rsl remote reconcile

Reconcile local RSL with remote RSL

### Synopsis

This command checks the local RSL against the specified remote and reconciles the local RSL if needed. If the local RSL doesn't exist or is strictly behind the remote RSL, then the local RSL is updated to match the remote RSL. If the local RSL is ahead of the remote RSL, nothing is updated. Finally, if the local and remote RSLs have diverged, then the local only RSL entries are reapplied over the latest entries in the remote if the local only RSL entries and remote only entries are for different Git references.

```
gittuf rsl remote reconcile <remote> [flags]
```

### Options

```
  -h, --help   help for reconcile
```

### Options inherited from parent commands

```
      --profile                      enable CPU and memory profiling
      --profile-CPU-file string      file to store CPU profile (default "cpu.prof")
      --profile-memory-file string   file to store memory profile (default "memory.prof")
      --verbose                      enable verbose logging
```

### SEE ALSO

* [gittuf rsl remote](gittuf_rsl_remote.md)	 - Tools for managing remote RSLs

