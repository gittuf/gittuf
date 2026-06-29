## gittuf trust remove-policy-key

Remove Policy key from gittuf root of trust

### Synopsis

The 'remove-policy-key' command removes a trusted key for the primary policy file from the repository's root of trust. It is used to revoke a key's authorization to sign policy metadata.

```
gittuf trust remove-policy-key [flags]
```

### Options

```
  -h, --help                   help for remove-policy-key
      --policy-key-ID string   ID of the policy key to remove from the root of trust
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

