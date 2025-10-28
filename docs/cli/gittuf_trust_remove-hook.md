## gittuf trust remove-hook

Remove a gittuf hook specified in the policy (developer mode only, set GITTUF_DEV=1)

### Synopsis

The 'remove-hook' command removes the specified gittuf hook from the repository's gittuf policy for the selected Git stages. This command can only be used in developer mode (set GITTUF_DEV=1) and supports removing hooks from the pre-commit or pre-push stages.

```
gittuf trust remove-hook [flags]
```

### Options

```
  -h, --help               help for remove-hook
      --hook-name string   name of hook
      --is-pre-commit      remove the hook from the pre-commit stage
      --is-pre-push        remove the hook from the pre-push stage
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

