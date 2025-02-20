## gittuf attest github pull-request

Record GitHub pull request information as an attestation

```
gittuf attest github pull-request [flags]
```

### Options

```
      --base-URL string           location of GitHub instance (default "https://github.com")
      --base-branch string        base branch for pull request, used with --commit
      --commit string             commit to record pull request attestation for
  -h, --help                      help for pull-request
      --pull-request-number int   pull request number to record in attestation (default -1)
      --repository string         path to base GitHub repository the pull request is opened against, of form {owner}/{repo}
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

