## gittuf trust add-propagation

Add propagation directive into gittuf root of trust (developer mode only, set GITTUF_DEV=1)

```
gittuf trust add-propagation [flags]
```

### Options

```
      --from-reference string    reference to propagate from in upstream repository
      --from-repository string   location of upstream repository
  -h, --help                     help for add-propagation
      --into-path string         path to propagate upstream contents into in downstream reference
      --into-reference string    reference to propagate into in downstream repository
```

### Options inherited from parent commands

```
      --no-color                     turn off colored output
      --profile                      enable CPU and memory profiling
      --profile-CPU-file string      file to store CPU profile (default "cpu.prof")
      --profile-memory-file string   file to store memory profile (default "memory.prof")
  -k, --signing-key string           signing key to use to sign root of trust
      --verbose                      enable verbose logging
```

### SEE ALSO

* [gittuf trust](gittuf_trust.md)	 - Tools for gittuf's root of trust

