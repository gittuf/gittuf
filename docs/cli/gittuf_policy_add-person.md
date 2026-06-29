## gittuf policy add-person

Add a trusted person to a policy file

### Synopsis

The 'add-person' command adds a trusted person to a gittuf policy file. It is used to define a person, along with their authorized keys and platform identities, who can then be named in policy rules.

```
gittuf policy add-person [flags]
```

### Options

```
      --associated-identity stringArray   identities on code review platforms in the form 'providerID::identity' (e.g., 'https://gittuf.dev/github-app::<username>+<user ID>')
      --custom stringArray                additional custom metadata in the form KEY=VALUE
  -h, --help                              help for add-person
      --person-ID string                  person ID
      --policy-name string                name of policy file to add person to (default "targets")
      --public-key stringArray            authorized public key for the person (path to SSH public key, "gpg:<fingerprint>" for GPG, or "fulcio:<identity>::<issuer>" for Sigstore)
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

