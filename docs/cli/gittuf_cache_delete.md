## gittuf cache delete

Delete the local persistent cache

### Synopsis

The 'delete' command deletes the local persistent cache used by gittuf. It is used to reclaim space or clear a stale cache. The cache must be reinitialized manually with 'gittuf cache init' before it can be used again.

```
gittuf cache delete [flags]
```

### Options

```
  -h, --help   help for delete
```

### Options inherited from parent commands

```
      --no-color                     turn off colored output
      --profile                      enable CPU and memory profiling
      --profile-CPU-file string      file to store CPU profile (default "cpu.prof")
      --profile-memory-file string   file to store memory profile (default "memory.prof")
      --storer-trace                 report Git storage backend call counts, timings and git fork counts on exit
      --storer-trace-file string     file to store the Git storage backend trace (default "storer.trace")
      --verbose                      enable verbose logging
```

### SEE ALSO

* [gittuf cache](gittuf_cache.md)	 - Manage gittuf's caching functionality

