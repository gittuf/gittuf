## gittuf verify-ref

Tools for verifying gittuf policies

```
gittuf verify-ref [flags]
```

### Options

```
      -- string                                       path to write a single source verification summary attestation
      --from-entry string                             perform verification from specified RSL entry (developer mode only, set GITTUF_DEV=1)
  -h, --help                                          help for verify-ref
      --latest-only                                   perform verification against latest entry in the RSL
      --remote-ref-name string                        name of remote reference, if it differs from the local name
      --sign-source-attestation string                signing key or identity for one or more source provenance or verification attestations
      --write-source-provenance-attestations string   path to write source provenance attestations (one per policy) as an in-toto attestation bundle
```

### Options inherited from parent commands

```
      --no-color                     turn off colored output
      --profile                      enable CPU and memory profiling
      --profile-CPU-file string      file to store CPU profile (default "cpu.prof")
      --profile-memory-file string   file to store memory profile (default "memory.prof")
      --verbose                      enable verbose logging
```

### SEE ALSO

* [gittuf](gittuf.md)	 - A security layer for Git repositories, powered by TUF

