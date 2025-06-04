## gittuf policy reorder-rules

Reorder rules in the specified policy file

### Synopsis

The 'reorder-rules' command reorders rules in the specified policy file based on the order of rule names provided as arguments. By default, the main policy file (targets) is used, which can be overridden with the '--policy-name' flag.

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

