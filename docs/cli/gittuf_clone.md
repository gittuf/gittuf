## gittuf clone

Clone repository and its gittuf references

### Synopsis

The 'clone' command clones a gittuf-enabled Git repository along with its associated gittuf metadata. It is used to obtain a repository and verify its RSL and policy against the provided root of trust keys.

```
gittuf clone [flags]
```

### Options

```
      --bare                   make a bare Git repository
  -b, --branch string          specify branch to check out
  -h, --help                   help for clone
      --root-key public-keys   set of initial root of trust keys for the repository (each a path to an SSH public key, "gpg:<fingerprint>" for GPG, or "fulcio:<identity>::<issuer>" for Sigstore)
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

