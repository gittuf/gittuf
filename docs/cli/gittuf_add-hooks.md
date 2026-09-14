## gittuf add-hooks

Add git hooks that automatically create and sync RSL

### Synopsis

The 'add-hooks' command installs Git hooks that automatically create and sync the RSL when certain Git actions occur, such as a push. It is used to keep the RSL up to date without running gittuf commands manually.

```
gittuf add-hooks [flags]
```

### Options

```
  -f, --force   overwrite hooks, if they already exist
  -h, --help    help for add-hooks
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

