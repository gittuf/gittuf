## gittuf policy increment-version

Increment the integer version of the specified policy file metadata

### Synopsis

The 'increment-version' command increments the integer version of the specified policy file metadata without making any other changes. This is normally only needed when upgrading gittuf metadata created with versions older than v0.14.0.

```
gittuf policy increment-version [flags]
```

### Options

```
  -h, --help                 help for increment-version
      --policy-name string   name of policy file to increment version of (default "targets")
```

### Options inherited from parent commands

```
      --create-rsl-entry             create RSL entry for policy change immediately (note: the RSL will not be synced with the remote)
      --no-color                     turn off colored output
      --profile                      enable CPU and memory profiling
      --profile-CPU-file string      file to store CPU profile (default "cpu.prof")
      --profile-memory-file string   file to store memory profile (default "memory.prof")
  -k, --signing-key string           signing key to use to sign policy metadata (path to SSH key, "gpg:<fingerprint>" for GPG, "fulcio:" for Sigstore)
      --verbose                      enable verbose logging
```

### SEE ALSO

* [gittuf policy](gittuf_policy.md)	 - Tools to manage gittuf policies

