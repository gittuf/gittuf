## gittuf trust add-github-app

Add GitHub app to gittuf root of trust

### Synopsis

The 'add-github-app' command adds a trusted key for the special GitHub app role to the repository's root of trust. It is used to verify signatures on GitHub pull request approval attestations.

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
  -k, --signing-key string           signing key to use to sign root of trust (path to SSH key, "gpg:<fingerprint>" for GPG, "fulcio:" for Sigstore)
      --verbose                      enable verbose logging
```

### SEE ALSO

* [gittuf trust](gittuf_trust.md)	 - Tools for gittuf's root of trust

