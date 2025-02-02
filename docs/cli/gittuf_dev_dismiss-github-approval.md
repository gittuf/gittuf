## gittuf dev dismiss-github-approval

Dismiss GitHub pull request approval as an attestation (developer mode only, set GITTUF_DEV=1)

```
gittuf dev dismiss-github-approval [flags]
```

### Options

```
      --base-URL string           location of GitHub instance (default "https://github.com")
      --dismiss-approver string   identity of the reviewer whose review was dismissed
  -h, --help                      help for dismiss-github-approval
      --review-ID int             pull request review ID (default -1)
  -k, --signing-key string        signing key to use for signing attestation
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

* [gittuf dev](gittuf_dev.md)	 - Developer mode commands

