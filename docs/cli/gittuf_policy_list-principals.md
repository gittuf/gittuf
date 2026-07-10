## gittuf policy list-principals

List principals for the current policy in the specified rule file

### Synopsis

The 'list-principals' command lists the trusted principals defined in a gittuf policy file. It is used to review which keys and persons are recognized before referencing them in rules.

```
gittuf policy list-principals [flags]
```

### Options

```
  -h, --help                 help for list-principals
      --policy-name string   specify rule file to list principals for (default "targets")
      --policy-ref string    specify which policy ref should be inspected (default "policy")
```

### Options inherited from parent commands

```
      --create-rsl-entry             create RSL entry for policy change immediately (note: the RSL will not be synced with the remote)
      --no-color                     turn off colored output
      --profile                      enable CPU and memory profiling
      --profile-CPU-file string      file to store CPU profile (default "cpu.prof")
      --profile-memory-file string   file to store memory profile (default "memory.prof")
  -k, --signing-key string           signing key to use to sign policy metadata (path to SSH key, "gpg:<fingerprint>" for GPG, "fulcio:" for Sigstore)
      --verbose                      enable verbose logging
```

### SEE ALSO

* [gittuf policy](gittuf_policy.md)	 - Tools to manage gittuf policies

