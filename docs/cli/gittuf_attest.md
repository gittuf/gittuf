## gittuf attest

Tools for attesting to code contributions

### Synopsis

The 'attest' command provides tools for attesting to code contributions. It includes subcommands to apply attestations, authorize contributors, and integrate GitHub-based attestations.

### Options

```
      --create-rsl-entry     create RSL entry for attestation change immediately (note: the new entry to the RSL will not be synced with the remote)
  -h, --help                 help for attest
  -k, --signing-key string   signing key to use to sign attestation
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
* [gittuf attest apply](gittuf_attest_apply.md)	 - Apply and push local attestations changes to remote repository
* [gittuf attest authorize](gittuf_attest_authorize.md)	 - Add or revoke reference authorization
* [gittuf attest github](gittuf_attest_github.md)	 - Tools to attest about GitHub actions and entities

