## gittuf trust remove-policy-key

Remove Policy key from gittuf root of trust

### Synopsis

This command allows users to remove a policy key from the gittuf root of trust. The policy key ID must be specified.

```
gittuf trust remove-policy-key [flags]
```

### Options

```
  -h, --help                   help for remove-policy-key
      --policy-key-ID string   ID of Policy key to be removed from root of trust
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

