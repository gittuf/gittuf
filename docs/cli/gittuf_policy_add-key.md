## gittuf policy add-key

Add a trusted key to a policy file

### Synopsis

The 'add-key' command adds one or more trusted public keys to a gittuf policy file. It is used to make keys available for use in policy rules. A key must be added to at least one rule before it can be used to authorize changes.

```
gittuf policy add-key [flags]
```

### Options

```
  -h, --help[=true|false]        help for add-key
      --policy-name string       name of policy file to add key to (default "targets")
      --public-key stringArray   authorized public key (path to SSH public key, "gpg:<fingerprint>" for GPG, or "fulcio:<identity>::<issuer>" for Sigstore)
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

