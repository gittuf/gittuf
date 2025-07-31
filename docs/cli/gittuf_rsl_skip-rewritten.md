## gittuf rsl skip-rewritten

Creates an RSL annotation to skip RSL reference entries that point to commits that do not exist in the specified ref

### Synopsis

The 'skip-rewritten' command adds an RSL annotation to skip reference entries that point to commits no longer present in the given Git reference, useful when the history of a branch has been rewritten and some RSL entries refer to commits that no longer exist on the branch.

```
gittuf rsl skip-rewritten [flags]
```

### Options

```
  -h, --help   help for skip-rewritten
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

* [gittuf rsl](gittuf_rsl.md)	 - Tools to manage the repository's reference state log

