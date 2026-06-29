## gittuf trust update-hook

Modify the parameters of an existing gittuf hook (developer mode only, set GITTUF_DEV=1)

### Synopsis

The 'update-hook' command modifies the parameters of an existing gittuf hook in the repository's root of trust. It is used to change a hook's script, environment, principals, or timeout. All parameters must be provided as when adding the hook, and only the stages given are updated; a stage where the hook does not already exist is left unchanged. The only currently supported environment is 'lua' (developer mode only, set GITTUF_DEV=1)

```
gittuf trust update-hook [flags]
```

### Options

```
  -e, --env string                 environment which the hook must run in (default "lua")
  -f, --file-path string           path of the script to be run as a hook
  -h, --help                       help for update-hook
  -n, --hook-name string           name of the hook
      --is-pre-commit              update the hook in the pre-commit stage
      --is-pre-push                update the hook in the pre-push stage
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
  -k, --signing-key string           signing key to use to sign root of trust (path to SSH key, "gpg:<fingerprint>" for GPG, "fulcio:" for Sigstore)
      --verbose                      enable verbose logging
```

### SEE ALSO

* [gittuf trust](gittuf_trust.md)	 - Tools for gittuf's root of trust

