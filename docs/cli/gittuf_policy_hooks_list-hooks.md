## gittuf policy hooks list-hooks

List hooks for the current policy in the specified rule file

```
gittuf policy hooks list-hooks [flags]
```

### Options

```
  -h, --help                 help for list-hooks
      --policy-name string   specify rule file to list principals for (default "targets")
      --target-ref string    specify which policy ref should be inspected (default "policy")
```

### Options inherited from parent commands

```
      --profile                      enable CPU and memory profiling
      --profile-CPU-file string      file to store CPU profile (default "cpu.prof")
      --profile-memory-file string   file to store memory profile (default "memory.prof")
  -k, --signing-key string           signing key to use to sign policy file
      --verbose                      enable verbose logging
```

### SEE ALSO

* [gittuf policy hooks](gittuf_policy_hooks.md)	 - Tools to manage git hooks

