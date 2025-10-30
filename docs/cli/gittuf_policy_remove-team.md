## gittuf policy remove-team

Remove an entire team from a policy file

### Synopsis

The 'remove-team' command removes the specified team from the specified gittuf policy file. By default, the main policy file (targets) is used, which can be overridden with the '--policy-name' flag.

```
gittuf policy remove-team [flags]
```

### Options

```
  -h, --help                 help for remove-team
      --policy-name string   name of policy file to remove team from (default "targets")
      --team-ID string       team ID
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

* [gittuf policy](gittuf_policy.md)	 - Tools to manage gittuf policies

