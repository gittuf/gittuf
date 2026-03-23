## gittuf trust make-controller

Make current repository a controller

### Synopsis

The 'make-controller' command sets the current repository as a controller in the repository's root of trust. It is used to allow downstream repositories to reuse this repository's policy.

```
gittuf trust make-controller [flags]
```

### Options

```
  -h, --help   help for make-controller
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

