## gittuf policy hooks remove

remove a hook specified in the policy

```
gittuf policy hooks remove [flags]
```

### Options

```
  -h, --help                 help for remove
      --hook-name string     name of hook
      --policy-name string   name of policy file to remove hook from (default "targets")
      --stage string         stage of hook
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

