## gittuf trust stage

Stage and push local policy-staging changes to remote repository

### Synopsis

The 'stage' command stages local policy changes and records them in the RSL. It optionally pushes the staged changes to a remote repository. It is used to prepare policy updates so they can be reviewed and signed by other users if needed. Pass '--local-only' to stage without pushing upstream. Otherwise, supply the remote name as the first positional argument.

```
gittuf trust stage [flags]
```

### Options

```
  -h, --help         help for stage
      --local-only   stage policy changes locally without pushing to a remote repository
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

