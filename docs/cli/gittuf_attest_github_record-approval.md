## gittuf attest github record-approval

Record GitHub pull request approval

### Synopsis

The 'record-approval' command creates an attestation for an approval action on a GitHub pull request. This command requires the repository in the {owner}/{repo} format, the pull request number, the specific review ID, and the identity of the reviewer who approved the pull request. The command also supports custom GitHub base URLs for enterprise GitHub instances, with the flag '--base-URL'.

```
gittuf attest github record-approval [flags]
```

### Options

```
      --approver string           identity of the reviewer who approved the change
      --base-URL string           location of GitHub instance (default "https://github.com")
  -h, --help                      help for record-approval
      --pull-request-number int   pull request number (default -1)
      --repository string         path to base GitHub repository the pull request is opened against, of form {owner}/{repo}
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

