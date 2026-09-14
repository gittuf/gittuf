## gittuf rsl remote push

Push RSL to the specified remote

### Synopsis

The 'push' command sends new entries in the local RSL to the specified remote repository. It is used to publish local RSL updates so they are available for other users to pull down.

```
gittuf rsl remote push <remote> [flags]
```

### Options

```
  -h, --help   help for push
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

* [gittuf rsl remote](gittuf_rsl_remote.md)	 - Tools for managing remote RSLs

