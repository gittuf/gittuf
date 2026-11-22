## gittuf trust add-controller-repository

Add a controller repository (developer mode only, set GITTUF_DEV=1)

### Synopsis

The 'add-controller-repository' command adds a controller repository to the repository's root of trust. It is used to designate a repository whose global rules this repository inherits (developer mode only, set GITTUF_DEV=1).

```
gittuf trust add-controller-repository [flags]
```

### Options

```
  -h, --help[=true|false]                    help for add-controller-repository
      --initial-root-principal stringArray   initial root principals of the controller repository (each a path to an SSH public key, "gpg:<fingerprint>" for GPG, or "fulcio:<identity>::<issuer>" for Sigstore)
      --location string                      location of controller repository
      --name string                          name of controller repository
```

### Options inherited from parent commands

```
      --create-rsl-entry[=true|false]   create RSL entry for policy change immediately (note: the RSL will not be synced with the remote)
      --no-color[=true|false]           turn off colored output
      --profile[=true|false]            enable CPU and memory profiling
      --profile-CPU-file string         file to store CPU profile (default "cpu.prof")
      --profile-memory-file string      file to store memory profile (default "memory.prof")
  -k, --signing-key string              signing key to use to sign root of trust (path to SSH key, "gpg:<fingerprint>" for GPG, "fulcio:" for Sigstore)
      --storer-trace[=true|false]       report Git storage backend call counts, timings and git fork counts on exit
      --storer-trace-file string        file to store the Git storage backend trace (default "storer.trace")
      --verbose[=true|false]            enable verbose logging
```

### SEE ALSO

* [gittuf trust](gittuf_trust.md)	 - Tools for gittuf's root of trust

