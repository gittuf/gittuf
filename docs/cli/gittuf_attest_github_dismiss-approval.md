## gittuf attest github dismiss-approval

Record dismissal of GitHub pull request approval

### Synopsis

The 'dismiss-approval' command creates an attestation that a previously recorded approval of a GitHub pull request has been dismissed. It requires the review ID of the pull request and the identity of the dismissed approver. The command also supports custom GitHub base URLs for enterprise GitHub instances, with the flag '--base-URL'.

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
      --create-rsl-entry             create RSL entry for attestation change immediately (note: the new entry to the RSL will not be synced with the remote)
      --no-color                     turn off colored output
      --profile                      enable CPU and memory profiling
      --profile-CPU-file string      file to store CPU profile (default "cpu.prof")
      --profile-memory-file string   file to store memory profile (default "memory.prof")
  -k, --signing-key string           signing key to use to sign attestation
      --verbose                      enable verbose logging
```

### SEE ALSO

* [gittuf attest github](gittuf_attest_github.md)	 - Tools to attest about GitHub actions and entities

