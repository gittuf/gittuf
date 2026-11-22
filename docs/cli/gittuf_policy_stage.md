## gittuf policy stage

Stage and push local policy-staging changes to remote repository

### Synopsis

The 'stage' command stages local policy changes and records them in the RSL. It optionally pushes the staged changes to a remote repository. It is used to prepare policy updates so they can be reviewed and signed by other users if needed. Pass '--local-only' to stage without pushing upstream. Otherwise, supply the remote name as the first positional argument.

```
gittuf policy stage [flags]
```

### Options

```
  -h, --help[=true|false]         help for stage
      --local-only[=true|false]   stage policy changes locally without pushing to a remote repository
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

