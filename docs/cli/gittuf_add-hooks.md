## gittuf add-hooks

Add git hooks that automatically create and sync RSL

### Synopsis

The 'add-hooks' command installs Git hooks that automatically create and sync the RSL when certain Git actions occur, such as a push. By default, it prevents overwriting existing hooks unless the '--force' flag is specified.

Supported hook types:
  - pre-push: Automatically creates RSL entries and syncs with remote before pushing
  - pre-commit: Validates staged changes against gittuf policies  
  - post-commit: Provides guidance on RSL management after commits

Examples:
  gittuf add-hooks                           # Install default pre-push hook
  gittuf add-hooks --hooks pre-push,pre-commit  # Install multiple hooks
  gittuf add-hooks --list                    # List installed hooks
  gittuf add-hooks --remove                  # Remove all gittuf hooks
  gittuf add-hooks --remove --hooks pre-push # Remove specific hook
  gittuf add-hooks --force                   # Force overwrite existing hooks

```
gittuf add-hooks [flags]
```

### Options

```
  -f, --force           overwrite hooks, if they already exist
  -h, --help            help for add-hooks
      --hooks strings   comma-separated list of hook types to install (pre-push, pre-commit, post-commit) (default [pre-push])
      --list            list installed gittuf hooks
      --remove          remove installed gittuf hooks
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

