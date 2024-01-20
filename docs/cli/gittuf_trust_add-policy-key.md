## gittuf trust add-policy-key

Add Policy key to gittuf root of trust

### Synopsis

This command allows users to add a new trusted key for the main policy file. Note that authorized keys can be specified from disk, from the GPG keyring using the "gpg:<fingerprint>" format, or as a Sigstore identity as "fulcio:<identity>::<issuer>".

```
gittuf trust add-policy-key [flags]
```

### Options

```
  -h, --help                help for add-policy-key
      --policy-key string   policy key to add to root of trust
```

### Options inherited from parent commands

```
      --profile                      enable CPU and memory profiling
      --profile-CPU-file string      file to store CPU profile (default "cpu.prof")
      --profile-memory-file string   file to store memory profile (default "memory.prof")
  -k, --signing-key string           signing key to use to sign root of trust
      --use-git-binary               use Git binary for some operations (developer mode only, set GITTUF_DEV=1)
      --verbose                      enable verbose logging
```

### SEE ALSO

* [gittuf trust](gittuf_trust.md)	 - Tools for gittuf's root of trust

