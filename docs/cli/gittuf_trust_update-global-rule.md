## gittuf trust update-global-rule

Update an existing global rule in the root of trust

### Synopsis

The 'update-global-rule' command updates an existing global rule in the repository's root of trust. It is used to adjust a repository-wide constraint, such as its patterns or signature threshold. A global rule can only be updated with the same rule type; changing the type requires removing and adding the rule again.

```
gittuf trust update-global-rule [flags]
```

### Options

```
  -h, --help                       help for update-global-rule
      --rule-name string           name of rule
      --rule-pattern stringArray   patterns used to identify namespaces rule applies to
      --threshold int              threshold of required valid signatures (default 1)
      --type string                type of rule (threshold|block-force-pushes)
```

### Options inherited from parent commands

```
      --create-rsl-entry             create RSL entry for policy change immediately (note: the RSL will not be synced with the remote)
      --no-color                     turn off colored output
      --profile                      enable CPU and memory profiling
      --profile-CPU-file string      file to store CPU profile (default "cpu.prof")
      --profile-memory-file string   file to store memory profile (default "memory.prof")
  -k, --signing-key string           signing key to use to sign root of trust (path to SSH key, "gpg:<fingerprint>" for GPG, "fulcio:" for Sigstore)
      --verbose                      enable verbose logging
```

### SEE ALSO

* [gittuf trust](gittuf_trust.md)	 - Tools for gittuf's root of trust

