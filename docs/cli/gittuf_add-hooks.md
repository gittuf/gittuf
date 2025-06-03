## gittuf add-hooks

Add git hooks that automatically create and sync RSL

### Synopsis

The 'add-hooks' command installs Git hooks that automatically create and sync the RSL when certain Git actions occur, such as a push. By default, it prevents overwriting existing hooks unless the '--force' flag is specified.

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
      --verbose                      enable verbose logging
```

### SEE ALSO

* [gittuf](gittuf.md)	 - A security layer for Git repositories, powered by TUF

