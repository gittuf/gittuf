## gittuf rsl remote reconcile

Reconcile local RSL with remote RSL

### Synopsis

The 'reconcile' command checks the local RSL against the specified remote and reconciles the local RSL if needed. It is used to bring the local RSL back in line with the remote after the two diverge. If the local RSL does not exist or is strictly behind the remote, it is updated to match the remote; if it is ahead, nothing is updated; and if the two have diverged, the local-only entries are reapplied over the latest remote entries when the local-only and remote-only entries are for different Git references.

```
gittuf rsl remote reconcile <remote> [flags]
```

### Options

```
  -h, --help   help for reconcile
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

* [gittuf rsl remote](gittuf_rsl_remote.md)	 - Tools for managing remote RSLs

