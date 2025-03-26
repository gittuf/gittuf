## gittuf trust update-global-rule

Update an existing global rule in the root of trust (developer mode only, set GITTUF_DEV=1)

### Synopsis

This command allows users to update an existing global rule in the root of trust. The name of the global rule must be specified. Note that a global rule may only be updated with the same type of global rule, and changes to the type require removing and adding it again.

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
  -k, --signing-key string           signing key to use to sign root of trust
      --verbose                      enable verbose logging
```

### SEE ALSO

* [gittuf trust](gittuf_trust.md)	 - Tools for gittuf's root of trust

