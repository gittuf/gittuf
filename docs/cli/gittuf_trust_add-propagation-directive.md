## gittuf trust add-propagation-directive

Add propagation directive into gittuf root of trust

### Synopsis

The 'add-propagation-directive' command registers a propagation directive in the repository's root of trust. It is used to define how contents from an upstream repository are propagated into a downstream repository by specifying source and destination references and paths.

```
gittuf trust add-propagation-directive [flags]
```

### Options

```
      --from-path string         path in upstream reference to propagate contents from
      --from-reference string    reference to propagate from in upstream repository
      --from-repository string   location of upstream repository
  -h, --help                     help for add-propagation-directive
      --into-path string         path to propagate upstream contents into in downstream reference
      --into-reference string    reference to propagate into in downstream repository
      --name string              name of propagation directive
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

