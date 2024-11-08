## gittuf policy add-person

Add a trusted person to a policy file (requires developer mode and v0.2 policy metadata to be enabled, set GITTUF_DEV=1 and GITTUF_ALLOW_V02_POLICY=1)

### Synopsis

This command allows users to add a trusted person to the specified policy file. By default, the main policy file is selected. Note that the person's keys can be specified from disk, from the GPG keyring using the "gpg:<fingerprint>" format, or as a Sigstore identity as "fulcio:<identity>::<issuer>".

```
gittuf policy add-person [flags]
```

### Options

```
      --associated-identity stringArray   identities on code review platforms in the form 'providerID::identity' (e.g., 'https://github.com::<username>')
      --custom stringArray                additional custom metadata in the form KEY=VALUE
  -h, --help                              help for add-person
      --person-ID string                  person ID
      --policy-name string                name of policy file to add key to (default "targets")
      --public-key stringArray            authorized public key for person
```

### Options inherited from parent commands

```
      --profile                      enable CPU and memory profiling
      --profile-CPU-file string      file to store CPU profile (default "cpu.prof")
      --profile-memory-file string   file to store memory profile (default "memory.prof")
  -k, --signing-key string           signing key to use to sign policy file
      --verbose                      enable verbose logging
```

### SEE ALSO

* [gittuf policy](gittuf_policy.md)	 - Tools to manage gittuf policies

