## gittuf trust update-hook

Modify the parameters of an existing gittuf hook, specify the hookname and the parameters to be updated. (developer mode only, set GITTUF_DEV=1)

### Synopsis

Modify the parameters of an existing gittuf hook. Specify the name of the hook to update and provide all parameters with their updated values. Note that all parameters required to add the hook must also be provided. Currently, only the 'lua' environment is supported (developer mode only, set GITTUF_DEV=1)

```
gittuf trust update-hook [flags]
```

### Options

```
  -e, --env string                 environment which the hook must run in (default "lua")
  -f, --file-path string           path of the script to be run as a hook
  -h, --help                       help for update-hook
  -n, --hook-name string           Name of the hook
      --is-pre-commit              update the hook to the pre-commit stage
      --is-pre-push                update the hook to the pre-push stage
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

