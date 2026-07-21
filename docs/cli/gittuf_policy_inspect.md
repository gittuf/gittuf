## gittuf policy inspect

Inspect policy metadata

### Synopsis

This command displays a gittuf policy (rule) file's metadata in a human-readable format. Use --policy-name to select which policy file to display (defaults to the primary 'targets' file), and --revision to inspect the metadata as it was recorded in a specific policy commit.

```
gittuf policy inspect [flags]
```

### Options

```
  -h, --help                 help for inspect
      --policy-name string   name of policy file to inspect (default "targets")
      --revision string      commit ID of the gittuf policy-staging ref to inspect (defaults to the current state)
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

