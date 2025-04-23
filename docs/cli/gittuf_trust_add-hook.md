## gittuf trust add-hook

Add a script to be run as a gittuf hook, specify when and where to run it (developer mode only, set GITTUF_DEV=1)

### Synopsis

Add a script to be run as a gittuf hook, specify when and which environment to run it in. The only currently supported environment is 'lua' (developer mode only, set GITTUF_DEV=1)

```
gittuf trust add-hook [flags]
```

### Options

```
  -e, --env string                 environment which the hook must run in (default "lua")
  -f, --file-path string           path of the script to be run as a hook
  -h, --help                       help for add-hook
  -n, --hook-name string           Name of the hook
      --is-pre-commit              add the hook to the pre-commit stage
      --is-pre-push                add the hook to the pre-push stage
      --principal-ID stringArray   principal IDs which must run this hook
      --timeout int                timeout for hook execution (default 100)
```

### Options inherited from parent commands

```
      --create-rsl-entry             create RSL entry for policy change immediately (note: the RSL will not be synced with the remote)
      --no-color                     turn off colored output
      --profile                      enable CPU and memory profiling
      --profile-CPU-file string      file to store CPU profile (default "cpu.prof")
      --profile-memory-file string   file to store memory profile (default "memory.prof")
  -k, --signing-key string           signing key to use to sign root of trust (path to SSH key, "fulcio:" for Sigstore)
      --verbose                      enable verbose logging
```

### SEE ALSO

* [gittuf trust](gittuf_trust.md)	 - Tools for gittuf's root of trust

