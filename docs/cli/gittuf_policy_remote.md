## gittuf policy remote

Tools for managing remote policies

### Synopsis

The 'remote' subcommand provides tools for pulling and pushing gittuf policy to and from remote repositories.

### Options

```
  -h, --help   help for remote
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

* [gittuf policy](gittuf_policy.md)	 - Tools to manage gittuf policies
* [gittuf policy remote pull](gittuf_policy_remote_pull.md)	 - Pull policy from the specified remote
* [gittuf policy remote push](gittuf_policy_remote_push.md)	 - Push policy to the specified remote

