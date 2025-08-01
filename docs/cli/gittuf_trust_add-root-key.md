## gittuf trust add-root-key

Add Root key to gittuf root of trust

### Synopsis

The 'add-root-key' command allows users to add a new root key to the repository's root of trust. This command facilitates the addition of an extra root key to the existing trusted root keys, enabling multiple root keys or key rotation.

```
gittuf trust add-root-key [flags]
```

### Options

```
  -h, --help              help for add-root-key
      --root-key string   root key to add to root of trust
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

