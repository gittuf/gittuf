## gittuf attest authorize

Add or revoke reference authorization

### Synopsis

Authorize or revoke permission to merge changes from one ref to another. Use '--from-ref' to specify the source reference.

```
gittuf attest authorize [flags]
```

### Options

```
  -f, --from-ref string   ref to authorize merging changes from
  -h, --help              help for authorize
  -r, --revoke            revoke existing authorization
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

