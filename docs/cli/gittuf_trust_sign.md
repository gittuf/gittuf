## gittuf trust sign

Sign root of trust

### Synopsis

The 'sign' command adds a signature to the repository's root of trust using the supplied signing key. It is used to meet the root of trust's signature threshold when multiple root keys are required.

```
gittuf trust sign [flags]
```

### Options

```
  -h, --help[=true|false]   help for sign
```

### Options inherited from parent commands

```
      --create-rsl-entry[=true|false]   create RSL entry for policy change immediately (note: the RSL will not be synced with the remote)
      --no-color[=true|false]           turn off colored output
      --profile[=true|false]            enable CPU and memory profiling
      --profile-CPU-file string         file to store CPU profile (default "cpu.prof")
      --profile-memory-file string      file to store memory profile (default "memory.prof")
  -k, --signing-key string              signing key to use to sign root of trust (path to SSH key, "gpg:<fingerprint>" for GPG, "fulcio:" for Sigstore)
      --storer-trace[=true|false]       report Git storage backend call counts, timings and git fork counts on exit
      --storer-trace-file string        file to store the Git storage backend trace (default "storer.trace")
      --verbose[=true|false]            enable verbose logging
```

### SEE ALSO

* [gittuf trust](gittuf_trust.md)	 - Tools for gittuf's root of trust

