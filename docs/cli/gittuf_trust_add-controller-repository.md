## gittuf trust add-controller-repository

Add a controller repository (developer mode only, set GITTUF_DEV=1)

```
gittuf trust add-controller-repository [flags]
```

### Options

```
  -h, --help                                 help for add-controller-repository
      --initial-root-principal stringArray   initial root principals of controller repository
      --location string                      location of controller repository
      --name string                          name of controller repository
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

