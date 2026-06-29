## gittuf policy init

Initialize policy file

### Synopsis

The 'init' command initializes a new gittuf policy file in the repository. It is used to create a policy file that can then define the principals and rules governing changes to protected namespaces.

```
gittuf policy init [flags]
```

### Options

```
  -h, --help                 help for init
      --policy-name string   name of policy file to create (default "targets")
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

