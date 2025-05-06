## gittuf policy list-principals

List principals for the current policy in the specified rule file

### Synopsis

This command retrieves and lists the authorized principals for the specified policy and rule. The user must specify the policy ref they wish to inspect, and the name of the rule file to retrieve the principals for.

```
gittuf policy list-principals [flags]
```

### Options

```
  -h, --help                 help for list-principals
      --policy-name string   specify rule file to list principals for (default "targets")
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

