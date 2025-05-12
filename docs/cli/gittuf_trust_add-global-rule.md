## gittuf trust add-global-rule

Add a new global rule to root of trust (developer mode only, set GITTUF_DEV=1)

### Synopsis

This command allows a user to add a new global rule to the root of trust. The user must specify the name, type, and rule pattern for the rule.

```
gittuf trust add-global-rule [flags]
```

### Options

```
  -h, --help                       help for add-global-rule
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
  -k, --signing-key string           signing key to use to sign root of trust (path to SSH key, "fulcio:" for Sigstore)
      --verbose                      enable verbose logging
```

### SEE ALSO

* [gittuf trust](gittuf_trust.md)	 - Tools for gittuf's root of trust

