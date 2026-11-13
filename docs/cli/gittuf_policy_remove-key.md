## gittuf policy remove-key

Remove a key from a policy file

### Synopsis

The 'remove-key' command removes a public key from a gittuf policy file. The key must first be removed from all rules that reference it before this command will succeed.

```
gittuf policy remove-key [flags]
```

### Options

```
  -h, --help[=true|false]    help for remove-key
      --policy-name string   name of policy file to remove key from (default "targets")
      --public-key string    public key ID to remove from the policy
```

### Options inherited from parent commands

```
      --create-rsl-entry[=true|false]   create RSL entry for policy change immediately (note: the RSL will not be synced with the remote)
      --no-color[=true|false]           turn off colored output
      --profile[=true|false]            enable CPU and memory profiling
      --profile-CPU-file string         file to store CPU profile (default "cpu.prof")
      --profile-memory-file string      file to store memory profile (default "memory.prof")
  -k, --signing-key string              signing key to use to sign policy metadata (path to SSH key, "gpg:<fingerprint>" for GPG, "fulcio:" for Sigstore)
      --storer-trace[=true|false]       report Git storage backend call counts, timings and git fork counts on exit
      --storer-trace-file string        file to store the Git storage backend trace (default "storer.trace")
      --verbose[=true|false]            enable verbose logging
```

### SEE ALSO

* [gittuf policy](gittuf_policy.md)	 - Tools to manage gittuf policies

