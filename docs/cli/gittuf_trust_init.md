## gittuf trust init

Initialize gittuf root of trust for repository

### Synopsis

The 'init' command initializes a gittuf root of trust for the current repository. This sets up the initial trusted root metadata and prepares the repository for gittuf policy operations. It can optionally specify the repository location and records initialization details in the repository's trust metadata.

```
gittuf trust init [flags]
```

### Options

```
  -h, --help              help for init
      --location string   location of repository
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

