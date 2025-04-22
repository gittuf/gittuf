## gittuf policy add-rule

Add a new rule to a policy file

### Synopsis

This command allows users to add a new rule to the specified policy file. By default, the main policy file is selected. Note that authorized keys can be specified from disk, from the GPG keyring using the "gpg:<fingerprint>" format, or as a Sigstore identity as "fulcio:<identity>::<issuer>".

```
gittuf policy add-rule [flags]
```

### Options

```
      --authorize stringArray      authorize the principal IDs for the rule
  -h, --help                       help for add-rule
      --policy-name string         name of policy file to add rule to (default "targets")
      --rule-name string           name of rule
      --rule-pattern stringArray   patterns used to identify namespaces rule applies to
      --threshold int              threshold of required valid signatures (default 1)
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

