## gittuf policy remove-key

Remove a key from a policy file

### Synopsis

This command allows users to remove keys from the specified policy file. The public key ID is required. By default, the main policy file is selected.

```
gittuf policy remove-key [flags]
```

### Options

```
  -h, --help                 help for remove-key
      --policy-name string   name of policy file to remove key from (default "targets")
      --public-key string    public key ID to remove from the policy
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

* [gittuf policy](gittuf_policy.md)	 - Tools to manage gittuf policies

