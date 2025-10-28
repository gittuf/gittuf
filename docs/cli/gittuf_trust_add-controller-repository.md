## gittuf trust add-controller-repository

Add a controller repository

### Synopsis

The 'add-controller-repository' command registers a new controller repository in the current repository's gittuf policy. You must specify the controller repository's name with --name, its location with --location, and at least one initial root principal with --initial-root-principal. Once added, the controller repository is trusted to supply policies to be applied on the local repository.

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

