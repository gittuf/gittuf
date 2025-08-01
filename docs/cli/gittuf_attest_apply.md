## gittuf attest apply

Apply and push local attestations changes to remote repository

### Synopsis

The 'apply' command records the latest state of gittuf attestations in the RSL and pushes them to the remote repository. Pass '--local-only' to record the attestation locally without pushing upstream. Otherwise, you must supply the remote name as the first positional argument.

```
gittuf attest apply [flags]
```

### Options

```
  -h, --help         help for apply
      --local-only   indicate that the attestation must be committed into the RSL only locally
```

### Options inherited from parent commands

```
      --create-rsl-entry             create RSL entry for attestation change immediately (note: the new entry to the RSL will not be synced with the remote)
      --no-color                     turn off colored output
      --profile                      enable CPU and memory profiling
      --profile-CPU-file string      file to store CPU profile (default "cpu.prof")
      --profile-memory-file string   file to store memory profile (default "memory.prof")
  -k, --signing-key string           signing key to use to sign attestation
      --verbose                      enable verbose logging
```

### SEE ALSO

* [gittuf attest](gittuf_attest.md)	 - Tools for attesting to code contributions

