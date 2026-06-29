## gittuf policy remove-person

Remove a person from a policy file

### Synopsis

The 'remove-person' command removes a trusted person from a gittuf policy file. The person must first be removed from all rules that reference them before this command will succeed.

```
gittuf policy remove-person [flags]
```

### Options

```
  -h, --help                 help for remove-person
      --person-ID string     person ID
      --policy-name string   name of policy file to remove person from (default "targets")
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

