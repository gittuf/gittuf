## gittuf policy reorder-rules

Reorder rules in the specified policy file

### Synopsis

This command allows users to reorder rules in the specified policy file. By default, the main policy file is selected. The rule names need to be passed as arguments, in the new order they must appear in, starting from the first to the last rule. Rule names may contain spaces, so they should be enclosed in quotes if necessary.

```
gittuf policy reorder-rules [flags]
```

### Options

```
  -h, --help                 help for reorder-rules
      --policy-name string   name of policy file to reorder rules in (default "targets")
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

