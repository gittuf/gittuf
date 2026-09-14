## gittuf trust init

Initialize gittuf root of trust for repository

### Synopsis

The 'init' command initializes the gittuf root of trust for a repository. It is used to initialize gittuf metadata with the user invoking the command trusted for root operations, and must be run before any other gittuf metadata command can be run.

```
gittuf trust init [flags]
```

### Options

```
  -h, --help              help for init
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
      --storer string                Git storage backend to use, one of binary or go-git (experimental, overrides GITTUF_STORER) (default "binary")
      --storer-trace                 report Git storage backend call counts, timings and git fork counts to stderr on exit
      --verbose                      enable verbose logging
```

### SEE ALSO

* [gittuf trust](gittuf_trust.md)	 - Tools for gittuf's root of trust

