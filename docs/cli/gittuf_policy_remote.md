## gittuf policy remote

Tools for managing remote policies

### Synopsis

The 'remote' subcommand provides tools for pulling and pushing gittuf policy to and from remote repositories.

### Options

```
  -h, --help[=true|false]   help for remote
```

### Options inherited from parent commands

```
      --create-rsl-entry[=true|false]   create RSL entry for policy change immediately (note: the RSL will not be synced with the remote)
      --no-color[=true|false]           turn off colored output
      --profile[=true|false]            enable CPU and memory profiling
      --profile-CPU-file string         file to store CPU profile (default "cpu.prof")
      --profile-memory-file string      file to store memory profile (default "memory.prof")
  -k, --signing-key string              signing key to use to sign policy metadata (path to SSH key, "gpg:<fingerprint>" for GPG, "fulcio:" for Sigstore)
      --storer-trace[=true|false]       report Git storage backend call counts, timings and git fork counts on exit
      --storer-trace-file string        file to store the Git storage backend trace (default "storer.trace")
      --verbose[=true|false]            enable verbose logging
```

### SEE ALSO

* [gittuf policy](gittuf_policy.md)	 - Tools to manage gittuf policies
* [gittuf policy remote pull](gittuf_policy_remote_pull.md)	 - Pull policy from the specified remote
* [gittuf policy remote push](gittuf_policy_remote_push.md)	 - Push policy to the specified remote

