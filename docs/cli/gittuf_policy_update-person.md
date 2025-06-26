## gittuf policy update-person

Update a person in a policy file

### Synopsis

This command allows users to update a person's information in the specified policy file. By default, the main policy file is selected. The command replaces the person's existing information with the new values provided. If a field is not specified, its existing value is not preserved.

```
gittuf policy update-person [flags]
```

### Options

```
      --associated-identity stringArray   identities on code review platforms in the form 'providerID::identity' (e.g., 'https://gittuf.dev/github-app::<username>+<user ID>')
      --custom stringArray                custom metadata in the form KEY=VALUE
  -h, --help                              help for update-person
      --person-ID string                  person ID to update
      --policy-name string                name of policy file to update person in (default "targets")
      --public-key stringArray            public keys for person
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

