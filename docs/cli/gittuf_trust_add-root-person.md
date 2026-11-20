## gittuf trust add-root-person

Add a trusted person to gittuf root of trust

### Synopsis

The 'add-root-person' command adds a trusted person, along with their authorized keys and platform identities, to the repository's root of trust.

```
gittuf trust add-root-person [flags]
```

### Options

```
      --associated-identity stringArray   identities on code review platforms in the form 'providerID::identity' (e.g., 'https://gittuf.dev/github-app::<username>+<user ID>')
      --custom stringArray                additional custom metadata in the form KEY=VALUE
  -h, --help                              help for add-root-person
      --person-ID string                  person ID
      --public-key stringArray            authorized public key for the person (path to SSH public key, "gpg:<fingerprint>" for GPG, or "fulcio:<identity>::<issuer>" for Sigstore)
```

### Options inherited from parent commands

```
      --create-rsl-entry             create RSL entry for policy change immediately (note: the RSL will not be synced with the remote)
      --no-color                     turn off colored output
      --profile                      enable CPU and memory profiling
      --profile-CPU-file string      file to store CPU profile (default "cpu.prof")
      --profile-memory-file string   file to store memory profile (default "memory.prof")
  -k, --signing-key string           signing key to use to sign root of trust (path to SSH key, "gpg:<fingerprint>" for GPG, "fulcio:" for Sigstore)
      --storer-trace                 report Git storage backend call counts, timings and git fork counts on exit
      --storer-trace-file string     file to store the Git storage backend trace (default "storer.trace")
      --verbose                      enable verbose logging
```

### SEE ALSO

* [gittuf trust](gittuf_trust.md)	 - Tools for gittuf's root of trust

