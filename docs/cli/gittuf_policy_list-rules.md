## gittuf policy list-rules

List rules for the current state

### Synopsis

The 'list-rules' command lists the rules defined in a gittuf policy, shown as a tree in evaluation order. It is used to review which principals are authorized over which namespaces.

```
gittuf policy list-rules [flags]
```

### Options

```
  -h, --help                help for list-rules
      --target-ref string   specify which policy ref should be inspected (default "policy")
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

