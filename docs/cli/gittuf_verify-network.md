## gittuf verify-network

Verify state of network repositories

### Synopsis

The 'verify-network' command verifies the state of network repositories configured in the repository's root of trust. It is used to check the integrity and consistency of network repositories against the expected trust configuration.

```
gittuf verify-network [flags]
```

### Options

```
  -h, --help[=true|false]   help for verify-network
```

### Options inherited from parent commands

```
      --no-color[=true|false]        turn off colored output
      --profile[=true|false]         enable CPU and memory profiling
      --profile-CPU-file string      file to store CPU profile (default "cpu.prof")
      --profile-memory-file string   file to store memory profile (default "memory.prof")
      --storer-trace[=true|false]    report Git storage backend call counts, timings and git fork counts on exit
      --storer-trace-file string     file to store the Git storage backend trace (default "storer.trace")
      --verbose[=true|false]         enable verbose logging
```

### SEE ALSO

* [gittuf](gittuf.md)	 - A security layer for Git repositories, powered by TUF

