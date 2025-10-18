## gittuf policy list-teams

List teams for the current policy in the specified rule file

### Synopsis

The 'list-teams' command lists all trusted teams defined in a gittuf policy rule file. By default, the main policy file (targets) is used, which can be overridden with the '--policy-name' flag.

```
gittuf policy list-teams [flags]
```

### Options

```
  -h, --help                 help for list-teams
      --policy-name string   specify rule file to list teams for (default "targets")
      --policy-ref string    specify which policy ref should be inspected (default "policy")
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

