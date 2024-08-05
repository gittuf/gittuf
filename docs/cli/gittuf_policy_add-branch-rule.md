## gittuf policy add-branch-rule

Add a new branch protection rule to a policy file

### Synopsis

This command allows users to add a new branch protection rule to the specified policy file. By default, the main policy file is selected. Note that authorized keys can be specified from disk, from the GPG keyring using the "gpg:<fingerprint>" format, or as a Sigstore identity as "fulcio:<identity>::<issuer>".

```
gittuf policy add-branch-rule [flags]
```

### Options

```
      --authorize-key stringArray   authorized public key for rule
      --branch stringArray          branches rule applies to
  -h, --help                        help for add-branch-rule
      --policy-name string          name of policy file to add rule to (default "targets")
      --rule-name string            name of rule
      --threshold int               threshold of required valid signatures (default 1)
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

