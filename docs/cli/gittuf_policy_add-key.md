## gittuf policy add-key

Add a trusted key to a policy file

### Synopsis

The 'add-key' command adds one or more trusted public keys to a gittuf policy file. This command is used to define which keys are authorized to sign commits or policy changes according to the repository's trust model. Note that authorized keys can be specified from disk, from the GPG keyring using the "gpg:<fingerprint>" format, or as a Sigstore identity as "fulcio:<identity>::<issuer>". By default, the main policy file (targets) is used, which can be overridden with the '--policy-name' flag.

```
gittuf policy add-key [flags]
```

### Options

```
  -h, --help                     help for add-key
      --policy-name string       name of policy file to add key to (default "targets")
      --public-key stringArray   authorized public key
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

* [gittuf policy](gittuf_policy.md)	 - Tools to manage gittuf policies

