## gittuf clone

Clone repository and its gittuf references

### Synopsis

The 'clone' command clones a gittuf-enabled Git repository along with its associated gittuf metadata. This command can also ensure the repository's trust root is established correctly by using specified root keys, optionally supplied using the --root-key flag. You may also specify a particular branch to check out with --branch and choose whether to create a bare repository using --bare.

```
gittuf clone [flags]
```

### Options

```
      --bare                   make a bare Git repository
  -b, --branch string          specify branch to check out
  -h, --help                   help for clone
      --root-key public-keys   set of initial root of trust keys for the repository (supported values: paths to SSH keys, GPG key fingerprints, Sigstore/Fulcio identities)
```

### Options inherited from parent commands

```
      --no-color                     turn off colored output
      --profile                      enable CPU and memory profiling
      --profile-CPU-file string      file to store CPU profile (default "cpu.prof")
      --profile-memory-file string   file to store memory profile (default "memory.prof")
      --verbose                      enable verbose logging
```

### SEE ALSO

* [gittuf](gittuf.md)	 - A security layer for Git repositories, powered by TUF

