## gittuf policy remove-rule

Remove rule from a policy file

### Synopsis

The 'remove-rule' command removes a rule from a gittuf policy file. It is used to stop protecting a namespace or to remove an authorization that is no longer needed.

```
gittuf policy remove-rule [flags]
```

### Options

```
  -h, --help                 help for remove-rule
      --policy-name string   name of policy file to remove rule from (default "targets")
      --rule-name string     name of rule
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

