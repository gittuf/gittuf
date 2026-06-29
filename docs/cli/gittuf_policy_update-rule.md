## gittuf policy update-rule

Update an existing rule in a policy file

### Synopsis

The 'update-rule' command updates an existing rule in a gittuf policy file. It is used to change the principals, patterns, or signature threshold that the rule enforces.

```
gittuf policy update-rule [flags]
```

### Options

```
      --authorize stringArray      authorize the principal IDs for the rule
  -h, --help                       help for update-rule
      --policy-name string         name of policy file to update rule in (default "targets")
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
  -k, --signing-key string           signing key to use to sign policy metadata (path to SSH key, "gpg:<fingerprint>" for GPG, "fulcio:" for Sigstore)
      --verbose                      enable verbose logging
```

### SEE ALSO

* [gittuf policy](gittuf_policy.md)	 - Tools to manage gittuf policies

