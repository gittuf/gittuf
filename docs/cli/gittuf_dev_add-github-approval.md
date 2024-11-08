## gittuf dev add-github-approval

Record GitHub pull request approval as an attestation (developer mode only, set GITTUF_DEV=1)

```
gittuf dev add-github-approval [flags]
```

### Options

```
      --approver string           identity of the reviewer who approved the change
      --base-URL string           location of GitHub instance (default "https://github.com")
  -h, --help                      help for add-github-approval
      --pull-request-number int   pull request number (default -1)
      --repository string         path to base GitHub repository the pull request is opened against, of form {owner}/{repo}
      --review-ID int             pull request review ID (default -1)
  -k, --signing-key string        signing key to use for signing attestation
```

### Options inherited from parent commands

```
      --profile                      enable CPU and memory profiling
      --profile-CPU-file string      file to store CPU profile (default "cpu.prof")
      --profile-memory-file string   file to store memory profile (default "memory.prof")
      --verbose                      enable verbose logging
```

### SEE ALSO

* [gittuf dev](gittuf_dev.md)	 - Developer mode commands

