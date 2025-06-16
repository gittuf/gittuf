## gittuf trust remove-global-rule

Remove a global rule from root of trust

### Synopsis

This command allows users to remove an existing global rule from the root of trust. The name of the global rule must be specified.

```
gittuf trust remove-global-rule [flags]
```

### Options

```
  -h, --help               help for remove-global-rule
      --rule-name string   name of rule
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

