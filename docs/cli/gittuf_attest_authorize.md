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
  -k, --signing-key string           signing key to use to sign attestations (path to SSH key, "gpg:<fingerprint>" for GPG, "fulcio:" for Sigstore)
      --storer string                Git storage backend to use, one of binary or go-git (experimental, overrides GITTUF_STORER) (default "binary")
      --storer-trace                 report Git storage backend call counts, timings and git fork counts to stderr on exit
      --verbose                      enable verbose logging
```

### SEE ALSO

* [gittuf attest](gittuf_attest.md)	 - Tools for attesting to code contributions

