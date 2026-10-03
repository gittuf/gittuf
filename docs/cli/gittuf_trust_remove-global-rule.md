## gittuf trust remove-global-rule

Remove a global rule from root of trust

### Synopsis

The 'remove-global-rule' command removes an existing global rule from the repository's root of trust. It is used to lift a repository-wide constraint that is no longer needed.

```
gittuf trust remove-global-rule [flags]
```

### Options

```
  -h, --help               help for remove-global-rule
      --rule-name string   name of rule
```

### Options inherited from parent commands

```
      --create-rsl-entry             create RSL entry for policy change immediately (note: the RSL will not be synced with the remote)
      --no-color                     turn off colored output
      --profile                      enable CPU and memory profiling
      --profile-CPU-file string      file to store CPU profile (default "cpu.prof")
      --profile-memory-file string   file to store memory profile (default "memory.prof")
  -k, --signing-key string           signing key to use to sign root of trust (path to SSH key, "gpg:<fingerprint>" for GPG, "fulcio:" for Sigstore)
      --storer-trace                 report Git storage backend call counts, timings and git fork counts on exit
      --storer-trace-file string     file to store the Git storage backend trace (default "storer.trace")
      --verbose                      enable verbose logging
```

### SEE ALSO

* [gittuf trust](gittuf_trust.md)	 - Tools for gittuf's root of trust

