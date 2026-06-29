## gittuf sync

Synchronize local references with remote references based on RSL

### Synopsis

The 'sync' command synchronizes the local repository with the remote by fetching and applying the latest gittuf metadata. It is used to ensure the local state is up to date with what has been pushed to the remote.

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

