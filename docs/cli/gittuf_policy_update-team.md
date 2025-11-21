## gittuf policy update-team

Update an existing trusted team in a policy file

### Synopsis

The 'update-team' command updates the principals or the theshold of an existing trusted team in a gittuf policy file. In gittuf, a team definition consists of a unique identifier ('--team-ID'), zero or more unique IDs for authorized team members ('--principal-IDs'), and a threshold. By default, the main policy file (targets) is used, which can be overridden with the '--policy-name' flag.

```
gittuf policy update-team [flags]
```

### Options

```
  -h, --help                       help for update-team
      --policy-name string         name of policy file to update team in (default "targets")
      --principalIDs stringArray   authorized principalIDs of this team
      --team-ID string             team ID
      --threshold int              threshold of required valid signatures (default 1)
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

