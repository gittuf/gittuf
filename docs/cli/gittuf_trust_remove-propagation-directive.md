## gittuf trust remove-propagation-directive

Remove propagation directive from gittuf root of trust

### Synopsis

The 'remove-propagation-directive' command removes a propagation directive from the repository's root of trust. It is used to delete an existing directive and stop the propagation of content between repositories.

```
gittuf trust remove-propagation-directive [flags]
```

### Options

```
  -h, --help          help for remove-propagation-directive
      --name string   name of propagation directive
```

### Options inherited from parent commands

```
      --create-rsl-entry             create RSL entry for policy change immediately (note: the RSL will not be synced with the remote)
      --no-color                     turn off colored output
      --profile                      enable CPU and memory profiling
      --profile-CPU-file string      file to store CPU profile (default "cpu.prof")
      --profile-memory-file string   file to store memory profile (default "memory.prof")
  -k, --signing-key string           signing key to use to sign root of trust (path to SSH key, "gpg:<fingerprint>" for GPG, "fulcio:" for Sigstore)
      --storer string                Git storage backend to use, one of binary or go-git (experimental, overrides GITTUF_STORER) (default "binary")
      --storer-trace                 report Git storage backend call counts, timings and git fork counts to stderr on exit
      --verbose                      enable verbose logging
```

### SEE ALSO

* [gittuf trust](gittuf_trust.md)	 - Tools for gittuf's root of trust

