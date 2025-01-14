## gittuf policy hooks add

add a script to be run as a hook, specify when and where to run it.

### Synopsis

add a script to be run as a hook, specify when and which environment to run it in. Environment can be among lua, gvisor, docker and local.

```
gittuf policy hooks add [flags]
```

### Options

```
  -e, --env string                 environment which the hook must run in (default "lua")
  -f, --file string                filepath of the script to be run as a hook
  -h, --help                       help for add
  -n, --hookname string            Name of the hook
      --modules stringArray        modules which the Lua hook must run
      --policy-name string         name of policy file to add hook to (default "targets")
      --principalIDs stringArray   principal IDs which must run this hook
  -s, --stage string               stage at which the hook must be run
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

