## gittuf attest github

Tools to attest about GitHub actions and entities

### Synopsis

The 'github' command provides tools to create attestations for actions and entities associated with GitHub, such as pull requests and approvals. It includes subcommands to record approval of a GitHub pull request. dismiss a previously recorded approval, and attest to metadata related to GitHub pull requests.

### Options

```
  -h, --help   help for github
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
* [gittuf attest github dismiss-approval](gittuf_attest_github_dismiss-approval.md)	 - Record dismissal of GitHub pull request approval
* [gittuf attest github pull-request](gittuf_attest_github_pull-request.md)	 - Record GitHub pull request information as an attestation
* [gittuf attest github record-approval](gittuf_attest_github_record-approval.md)	 - Record GitHub pull request approval

