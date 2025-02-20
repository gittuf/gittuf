## gittuf attest github dismiss-approval

Record dismissal of GitHub pull request approval

```
gittuf attest github dismiss-approval [flags]
```

### Options

```
      --base-URL string           location of GitHub instance (default "https://github.com")
      --dismiss-approver string   identity of the reviewer whose review was dismissed
  -h, --help                      help for dismiss-approval
      --review-ID int             pull request review ID (default -1)
```

### Options inherited from parent commands

```
      --no-color                     turn off colored output
      --profile                      enable CPU and memory profiling
      --profile-CPU-file string      file to store CPU profile (default "cpu.prof")
      --profile-memory-file string   file to store memory profile (default "memory.prof")
  -k, --signing-key string           signing key to use to sign attestation
      --verbose                      enable verbose logging
```

### SEE ALSO

* [gittuf attest github](gittuf_attest_github.md)	 - Tools to attest about GitHub actions and entities

