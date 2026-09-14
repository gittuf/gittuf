## gittuf verify-ref

Tools for verifying gittuf policies

```
gittuf verify-ref [flags]
```

### Options

```
      --from-entry string        perform verification from specified RSL entry (developer mode only, set GITTUF_DEV=1)
  -h, --help                     help for verify-ref
      --latest-only              perform verification against latest entry in the RSL
      --remote-ref-name string   name of remote reference, if it differs from the local name
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

