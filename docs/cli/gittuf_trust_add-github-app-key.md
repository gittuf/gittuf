## gittuf trust add-github-app-key

Add GitHub app key to gittuf root of trust

### Synopsis

This command allows users to add a trusted key for the special GitHub app role. Note that authorized keys can be specified from disk, from the GPG keyring using the "gpg:<fingerprint>" format, or as a Sigstore identity as "fulcio:<identity>::<issuer>".

```
gittuf trust add-github-app-key [flags]
```

### Options

```
      --app-key string   app key to add to root of trust
  -h, --help             help for add-github-app-key
```

### Options inherited from parent commands

```
      --profile                      enable CPU and memory profiling
      --profile-CPU-file string      file to store CPU profile (default "cpu.prof")
      --profile-memory-file string   file to store memory profile (default "memory.prof")
  -k, --signing-key string           signing key to use to sign root of trust
      --verbose                      enable verbose logging
```

### SEE ALSO

* [gittuf trust](gittuf_trust.md)	 - Tools for gittuf's root of trust

