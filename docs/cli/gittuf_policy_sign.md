## gittuf policy sign

Sign policy file

### Synopsis

The 'sign' command adds a signature to a gittuf policy file using the supplied signing key. It is used to meet a policy file's signature threshold when multiple keys are required to approve it.

```
gittuf policy sign [flags]
```

### Options

```
  -h, --help                 help for sign
      --policy-name string   name of policy file to sign (default "targets")
```

### Options inherited from parent commands

```
      --create-rsl-entry             create RSL entry for policy change immediately (note: the RSL will not be synced with the remote)
      --no-color                     turn off colored output
      --profile                      enable CPU and memory profiling
      --profile-CPU-file string      file to store CPU profile (default "cpu.prof")
      --profile-memory-file string   file to store memory profile (default "memory.prof")
  -k, --signing-key string           signing key to use to sign policy metadata (path to SSH key, "gpg:<fingerprint>" for GPG, "fulcio:" for Sigstore)
      --verbose                      enable verbose logging
```

### SEE ALSO

* [gittuf policy](gittuf_policy.md)	 - Tools to manage gittuf policies

