## gittuf cache

Manage gittuf's caching functionality

### Synopsis

The 'cache' command group contains subcommands to manage gittuf's local persistent cache. This cache helps improve performance by storing metadata locally. The cache is local-only and is not synchronized with remote repositories.

### Options

```
  -h, --help   help for cache
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
* [gittuf cache delete](gittuf_cache_delete.md)	 - Delete the local persistent cache
* [gittuf cache init](gittuf_cache_init.md)	 - Initialize persistent cache

