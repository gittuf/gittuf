## gittuf sync

Synchronize local references with remote references based on RSL

### Synopsis

The 'sync' command synchronizes local references with the remote references based on the RSL (Reference State Log). By default, it uses the 'origin' remote unless a different remote name is provided. If references have diverged, it prints the list of affected refs and suggests rerunning the command with --overwrite to apply remote changes. Use with caution: --overwrite may discard local changes.

```
gittuf sync [remoteName] [flags]
```

### Options

```
  -h, --help        help for sync
      --overwrite   overwrite local references with upstream changes
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

