## gittuf trust hooks add

Add a script to be run as a gittuf hook, specify when and where to run it.

### Synopsis

Add a script to be run as a gittuf hook, specify when and which environment to run it in. The only currently supported environment is 'lua'.

```
gittuf trust hooks add [flags]
```

### Options

```
  -e, --env string                 environment which the hook must run in (default "lua")
  -f, --file string                filepath of the script to be run as a hook
  -h, --help                       help for add
  -n, --hookname string            Name of the hook
      --modules stringArray        modules which the Lua hook must run
      --principalIDs stringArray   principal IDs which must run this hook
  -s, --stage string               stage at which the hook must be run
```

### Options inherited from parent commands

```
      --create-rsl-entry             create RSL entry for policy change immediately (note: the RSL will not be synced with the remote)
      --no-color                     turn off colored output
      --profile                      enable CPU and memory profiling
      --profile-CPU-file string      file to store CPU profile (default "cpu.prof")
      --profile-memory-file string   file to store memory profile (default "memory.prof")
  -k, --signing-key string           signing key to use to sign root of trust
      --verbose                      enable verbose logging
```

### SEE ALSO

* [gittuf trust hooks](gittuf_trust_hooks.md)	 - Tools to manage gittuf hooks

