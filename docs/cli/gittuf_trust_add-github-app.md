## gittuf trust add-github-app

Add GitHub app to gittuf root of trust

### Synopsis

This command allows users to add a trusted key for the special GitHub app role. This key is used to verify signatures on GitHub pull request approval attestations. Note that authorized keys can be specified from disk, from the GPG keyring using the "gpg:<fingerprint>" format, or as a Sigstore identity as "fulcio:<identity>::<issuer>".

```
gittuf trust add-github-app [flags]
```

### Options

```
      --app-key string    app key to add to root of trust (path to SSH key, "fulcio:<identity>::<issuer>" for Sigstore, "gpg:<fingerprint>" for GPG key)
      --app-name string   name of app to add to root of trust (default "https://gittuf.dev/github-app")
  -h, --help              help for add-github-app
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

* [gittuf trust](gittuf_trust.md)	 - Tools for gittuf's root of trust

