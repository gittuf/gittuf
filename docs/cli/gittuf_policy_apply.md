## gittuf policy apply

Validate and apply changes from policy-staging to policy

### Synopsis

The 'apply' command validates and applies changes from the policy-staging area to the repository's policy. It is used to make staged policy updates effective and records the change in the RSL. Pass '--local-only' to apply without pushing upstream. Otherwise, supply the remote name as the first positional argument.

```
gittuf policy apply [flags]
```

### Options

```
  -h, --help         help for apply
      --local-only   apply policy changes locally without pushing to a remote repository
```

### Options inherited from parent commands

```
      --create-rsl-entry             create RSL entry for policy change immediately (note: the RSL will not be synced with the remote)
      --no-color                     turn off colored output
      --profile                      enable CPU and memory profiling
      --profile-CPU-file string      file to store CPU profile (default "cpu.prof")
      --profile-memory-file string   file to store memory profile (default "memory.prof")
  -k, --signing-key string           signing key to use to sign policy metadata (path to SSH key, "gpg:<fingerprint>" for GPG, "fulcio:" for Sigstore)
      --verbose                      enable verbose logging
```

### SEE ALSO

* [gittuf policy](gittuf_policy.md)	 - Tools to manage gittuf policies

