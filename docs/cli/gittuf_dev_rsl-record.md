## gittuf dev rsl-record

Record explicit state of a Git reference in the RSL, signed with specified key (developer mode only, set GITTUF_DEV=1)

### Synopsis

The 'rsl-record' command records the explicit state of a Git reference into the RSL. This command is intended for developer and testing workflows where manual creation of entries in the RSL for a specific Git reference (e.g., branch or tag) at a given commit (target ID) is required. You can optionally specify a destination reference name using --dst-ref to override the default. It requires developer mode to be enabled by setting the environment variable GITTUF_DEV=1.

```
gittuf dev rsl-record [flags]
```

### Options

```
      --dst-ref string       name of destination reference, if it differs from source reference
  -h, --help                 help for rsl-record
  -k, --signing-key string   path to PEM encoded SSH or GPG signing key
  -t, --target string        target ID
```

### Options inherited from parent commands

```
      --no-color                     turn off colored output
      --profile                      enable CPU and memory profiling
      --profile-CPU-file string      file to store CPU profile (default "cpu.prof")
      --profile-memory-file string   file to store memory profile (default "memory.prof")
      --verbose                      enable verbose logging
```

### SEE ALSO

* [gittuf dev](gittuf_dev.md)	 - Developer mode commands

