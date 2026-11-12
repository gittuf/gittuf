## gittuf verify-ref

Tools for verifying gittuf policies

```
gittuf verify-ref [flags]
```

### Options

```
      --from-entry string          perform verification from specified RSL entry (developer mode only, set GITTUF_DEV=1)
  -h, --help[=true|false]          help for verify-ref
      --latest-only[=true|false]   perform verification against latest entry in the RSL
      --remote-ref-name string     name of remote reference, if it differs from the local name
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

