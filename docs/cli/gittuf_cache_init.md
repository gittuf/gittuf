## gittuf cache init

Initialize persistent cache

### Synopsis

The 'init' command initializes the local persistent cache for a gittuf repository, intended to improve performance of gittuf operations. This cache is local-only and is not synchronized with the remote.

```
gittuf cache init [flags]
```

### Options

```
  -h, --help   help for init
```

### Options inherited from parent commands

```
      --no-color                     turn off colored output
      --profile                      enable CPU and memory profiling
      --profile-CPU-file string      file to store CPU profile (default "cpu.prof")
      --profile-memory-file string   file to store memory profile (default "memory.prof")
      --storer string                Git storage backend to use, one of binary or go-git (experimental, overrides GITTUF_STORER) (default "binary")
      --storer-trace                 report Git storage backend call counts, timings and git fork counts to stderr on exit
      --verbose                      enable verbose logging
```

### SEE ALSO

* [gittuf cache](gittuf_cache.md)	 - Manage gittuf's caching functionality

