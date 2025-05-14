## gittuf trust inspect-root

Print root metadata in JSON format

### Synopsis

This command prints the root metadata in a pretty-printed JSON format. By default, it inspects the policy ref, but you can specify a different policy ref using --target-ref.

```
gittuf trust inspect-root [flags]
```

### Options

```
  -h, --help                help for inspect-root
      --target-ref string   specify which policy ref should be inspected (default "policy")
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

