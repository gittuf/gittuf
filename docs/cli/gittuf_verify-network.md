## gittuf verify-network

Verify state of network repositories

### Synopsis

The 'verify-network' command verifies the state of network repositories configured in the repository's root of trust. It is used to check the integrity and consistency of network repositories against the expected trust configuration.

```
gittuf verify-network [flags]
```

### Options

```
  -h, --help   help for verify-network
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

* [gittuf](gittuf.md)	 - A security layer for Git repositories, powered by TUF

