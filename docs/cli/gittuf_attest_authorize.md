## gittuf attest authorize

Add or revoke reference authorization

### Synopsis

Authorize or revoke permission to merge changes from one ref to another. Use '--from-ref' to specify the source reference.

```
gittuf attest authorize [flags]
```

### Options

```
  -f, --from-ref string       ref to authorize merging changes from
  -h, --help[=true|false]     help for authorize
  -r, --revoke[=true|false]   revoke existing authorization
```

### Options inherited from parent commands

```
      --create-rsl-entry[=true|false]   create RSL entry for attestation change immediately (note: the new entry to the RSL will not be synced with the remote)
      --no-color[=true|false]           turn off colored output
      --profile[=true|false]            enable CPU and memory profiling
      --profile-CPU-file string         file to store CPU profile (default "cpu.prof")
      --profile-memory-file string      file to store memory profile (default "memory.prof")
  -k, --signing-key string              signing key to use to sign attestations (path to SSH key, "gpg:<fingerprint>" for GPG, "fulcio:" for Sigstore)
      --storer-trace[=true|false]       report Git storage backend call counts, timings and git fork counts on exit
      --storer-trace-file string        file to store the Git storage backend trace (default "storer.trace")
      --verbose[=true|false]            enable verbose logging
```

### SEE ALSO

* [gittuf attest](gittuf_attest.md)	 - Tools for attesting to code contributions

