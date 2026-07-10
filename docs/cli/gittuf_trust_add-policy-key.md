## gittuf trust add-policy-key

Add Policy key to gittuf root of trust

### Synopsis

The 'add-policy-key' command adds a new trusted key for the primary policy file to the repository's root of trust. It is used to authorize additional keys to sign the main policy metadata.

```
gittuf trust add-policy-key [flags]
```

### Options

```
  -h, --help                help for add-policy-key
      --policy-key string   policy key to add (path to SSH public key, "gpg:<fingerprint>" for GPG, or "fulcio:<identity>::<issuer>" for Sigstore)
```

### Options inherited from parent commands

```
      --create-rsl-entry             create RSL entry for policy change immediately (note: the RSL will not be synced with the remote)
      --no-color                     turn off colored output
      --profile                      enable CPU and memory profiling
      --profile-CPU-file string      file to store CPU profile (default "cpu.prof")
      --profile-memory-file string   file to store memory profile (default "memory.prof")
  -k, --signing-key string           signing key to use to sign root of trust (path to SSH key, "gpg:<fingerprint>" for GPG, "fulcio:" for Sigstore)
      --verbose                      enable verbose logging
```

### SEE ALSO

* [gittuf trust](gittuf_trust.md)	 - Tools for gittuf's root of trust

