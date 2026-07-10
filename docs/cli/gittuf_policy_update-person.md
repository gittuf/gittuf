## gittuf policy update-person

Update a person in a policy file

### Synopsis

The 'update-person' command updates a trusted person's definition in a gittuf policy file. It is used to change a person's keys, associated identities, or custom metadata. The update replaces the person entirely, so any field not provided is cleared rather than preserved.

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
      --public-key stringArray            public keys for the person (each a path to an SSH public key, "gpg:<fingerprint>" for GPG, or "fulcio:<identity>::<issuer>" for Sigstore)
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

