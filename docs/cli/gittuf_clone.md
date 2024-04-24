## gittuf clone

Clone repository and its gittuf references

```
gittuf clone [flags]
```

### Options

```
  -b, --branch string          specify branch to check out
  -h, --help                   help for clone
      --root-key public-keys   set of initial root of trust keys for the repository (supported values: paths to SSH keys, GPG key fingerprints, Sigstore/Fulcio identities)
```

### Options inherited from parent commands

```
      --profile                      enable CPU and memory profiling
      --profile-CPU-file string      file to store CPU profile (default "cpu.prof")
      --profile-memory-file string   file to store memory profile (default "memory.prof")
      --verbose                      enable verbose logging
```

### SEE ALSO

* [gittuf](gittuf.md)	 - A security layer for Git repositories, powered by TUF

