## gittuf policy remote push

Push policy to the specified remote

### Synopsis

The 'push' command sends the repository's policy to the specified remote. It is used to publish local policy changes so that they are available for other users to pull down.

```
gittuf policy remote push <remote> [flags]
```

### Options

```
  -h, --help   help for push
```

### Options inherited from parent commands

```
      --create-rsl-entry             create RSL entry for policy change immediately (note: the RSL will not be synced with the remote)
      --no-color                     turn off colored output
      --profile                      enable CPU and memory profiling
      --profile-CPU-file string      file to store CPU profile (default "cpu.prof")
      --profile-memory-file string   file to store memory profile (default "memory.prof")
  -k, --signing-key string           signing key to use to sign policy metadata (path to SSH key, "gpg:<fingerprint>" for GPG, "fulcio:" for Sigstore)
      --storer string                Git storage backend to use, one of binary or go-git (experimental, overrides GITTUF_STORER) (default "binary")
      --storer-trace                 report Git storage backend call counts, timings and git fork counts to stderr on exit
      --verbose                      enable verbose logging
```

### SEE ALSO

* [gittuf policy remote](gittuf_policy_remote.md)	 - Tools for managing remote policies

