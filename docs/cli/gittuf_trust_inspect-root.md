## gittuf trust inspect-root

Inspect root metadata

### Synopsis

This command displays the root metadata in a human-readable format. By default, the current state is shown; use --revision to inspect the metadata as it was recorded in a specific policy commit.

```
gittuf trust inspect-root [flags]
```

### Options

```
  -h, --help              help for inspect-root
      --revision string   commit ID of the gittuf policy-staging ref to inspect (defaults to the current state)
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

