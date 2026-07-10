## gittuf trust set-repository-location

Set repository location

### Synopsis

The 'set-repository-location' command records the canonical location of the repository in its root of trust. It is used to tell other repositories in a gittuf network where to fetch this repository from.

```
gittuf trust set-repository-location [flags]
```

### Options

```
  -h, --help              help for set-repository-location
      --location string   canonical location of the repository to record in the root of trust
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

