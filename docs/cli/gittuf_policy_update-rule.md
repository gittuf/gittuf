## gittuf policy update-rule

Update an existing rule in a policy file

### Synopsis

This command allows users to update an existing rule to the specified policy file. By default, the main policy file is selected. Note that authorized keys can be specified from disk, from the GPG keyring using the "gpg:<fingerprint>" format, or as a Sigstore identity as "fulcio:<identity>::<issuer>".

```
gittuf policy update-rule [flags]
```

### Options

```
      --authorize-key stringArray   authorized public key for rule
  -h, --help                        help for update-rule
      --policy-name string          name of policy file to add rule to (default "targets")
      --rule-name string            name of rule
      --rule-pattern stringArray    patterns used to identify namespaces rule applies to
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

