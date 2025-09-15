## gittuf policy update-team

Update an existing trusted team (person) in a policy file

### Synopsis

The 'update-team' command updates an existing trusted team/person in a gittuf policy file. A person is defined by a unique ID ('--person-ID'), authorized public keys ('--public-key'), optional associated identities ('--associated-identity') from external systems, and custom metadata ('--custom'). This command replaces the full person record (keys, identities, metadata) in the policy.

```
gittuf policy update-team [flags]
```

### Options

```
      --associated-identity stringArray   identities on code review platforms in the form 'providerID::identity' (e.g., 'https://gittuf.dev/github-app::<username>+<user ID>')
      --custom stringArray                additional custom metadata in the form KEY=VALUE
  -h, --help                              help for update-team
      --person-ID string                  person ID
      --policy-name string                name of policy file to update team in (default "targets")
      --public-key stringArray            authorized public key for person
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

