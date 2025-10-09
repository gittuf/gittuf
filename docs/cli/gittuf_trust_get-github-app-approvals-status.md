## gittuf trust get-github-app-approvals-status

Get whether GitHub App approvals are trusted

### Synopsis

Get whether GitHub App approvals are trusted for each GitHub app, from the repository's policy.

```
gittuf trust get-github-app-approvals-status [flags]
```

### Options

```
  -h, --help   help for get-github-app-approvals-status
```

### Options inherited from parent commands

```
      --create-rsl-entry             create RSL entry for policy change immediately (note: the RSL will not be synced with the remote)
      --no-color                     turn off colored output
      --profile                      enable CPU and memory profiling
      --profile-CPU-file string      file to store CPU profile (default "cpu.prof")
      --profile-memory-file string   file to store memory profile (default "memory.prof")
  -k, --signing-key string           signing key to use to sign root of trust (path to SSH key, "fulcio:" for Sigstore)
      --verbose                      enable verbose logging
```

### SEE ALSO

* [gittuf trust](gittuf_trust.md)	 - Tools for gittuf's root of trust

