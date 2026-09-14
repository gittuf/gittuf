## gittuf verify-mergeable

Tools for verifying mergeability using gittuf policies

### Synopsis

The 'verify-mergeable' command evaluates whether a feature branch can be merged into a base branch under the repository's gittuf policies. It is used to check, before performing a merge, that the proposed merge would satisfy policy such as required approval thresholds.

```
gittuf verify-mergeable [flags]
```

### Options

```
      --base-branch string      base branch for proposed merge
      --bypass-RSL              bypass RSL when identifying current state of feature ref
      --feature-branch string   feature branch for proposed merge
  -h, --help                    help for verify-mergeable
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

