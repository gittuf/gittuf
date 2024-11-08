## gittuf policy add-key

Add a trusted key to a policy file

### Synopsis

This command allows users to add trusted keys to the specified policy file. By default, the main policy file is selected. Note that the keys can be specified from disk, from the GPG keyring using the "gpg:<fingerprint>" format, or as a Sigstore identity as "fulcio:<identity>::<issuer>".

```
gittuf policy add-key [flags]
```

### Options

```
  -h, --help                     help for add-key
      --policy-name string       name of policy file to add key to (default "targets")
      --public-key stringArray   authorized public key
```

### Options inherited from parent commands

```
      --profile                      enable CPU and memory profiling
      --profile-CPU-file string      file to store CPU profile (default "cpu.prof")
      --profile-memory-file string   file to store memory profile (default "memory.prof")
  -k, --signing-key string           signing key to use to sign policy file
      --verbose                      enable verbose logging
```

### SEE ALSO

* [gittuf policy](gittuf_policy.md)	 - Tools to manage gittuf policies

