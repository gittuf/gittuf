## gittuf policy add-person

Add a trusted person to a policy file

### Synopsis

The 'add-person' command adds a trusted person to a gittuf policy file. In gittuf, a person definition consists of a unique identifier ('--person-ID'), one or more authorized public keys ('--public-key'), optional associated identities ('--associated-identity') on external platforms (e.g., GitHub, GitLab), and optional custom metadata ('--custom') for tracking additional attributes. Note that the keys can be specified from disk, from the GPG keyring using the "gpg:<fingerprint>" format, or as a Sigstore identity as "fulcio:<identity>::<issuer>". By default, the main policy file (targets) is used, which can be overridden with the '--policy-name' flag.

```
gittuf policy add-person [flags]
```

### Options

```
      --associated-identity stringArray   identities on code review platforms in the form 'providerID::identity' (e.g., 'https://gittuf.dev/github-app::<username>+<user ID>')
      --custom stringArray                additional custom metadata in the form KEY=VALUE
  -h, --help                              help for add-person
      --person-ID string                  person ID
      --policy-name string                name of policy file to add key to (default "targets")
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

