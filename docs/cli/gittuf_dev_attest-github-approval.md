## gittuf dev attest-github-approval

Record GitHub pull request approval as an attestation (developer mode only, set GITTUF_DEV=1)

```
gittuf dev attest-github-approval [flags]
```

### Options

```
      --approver string      approver signing key (path for SSH, gpg:<fingerprint> for GPG) / identity (fulcio:identity::provider)
      --base-branch string   base branch for pull request
      --from from            from revision ID--current tip of the base branch
  -h, --help                 help for attest-github-approval
  -k, --signing-key string   signing key to use for signing attestation
      --to to                to tree ID--the resultant Git tree when this pull request is merged
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

