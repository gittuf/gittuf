## gittuf dev populate-cache

Populate persistent cache (developer mode only, set GITTUF_DEV=1)

### Synopsis

The 'populate-cache' command generates and populates the local persistent cache for a gittuf repository, intended to improve performance of gittuf operations. This cache is local-only and is not synchronzied with the remote. It requires developer mode to be enabled by setting the environment variable GITTUF_DEV=1.

```
gittuf dev populate-cache [flags]
```

### Options

```
  -h, --help   help for populate-cache
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

* [gittuf dev](gittuf_dev.md)	 - Developer mode commands

