## gittuf verify-ref

Tools for verifying gittuf policies

```
gittuf verify-ref [flags]
```

### Options

```
      --from-entry string       perform verification from specified RSL entry (developer mode only, set GITTUF_DEV=1)
  -h, --help                    help for verify-ref
      --latest-only             perform verification against latest entry in the RSL
      --skip-file-policies      skip file policies (developer mode only, set GITTUF_DEV=1)
      --use-policy-path-cache   use policy path cache during verification (developer mode only, set GITTUF_DEV=1)
```

### Options inherited from parent commands

```
      --profile                      enable CPU and memory profiling
      --profile-CPU-file string      file to store CPU profile (default "cpu.prof")
      --profile-memory-file string   file to store memory profile (default "memory.prof")
      --use-git-binary               use Git binary for some operations (developer mode only, set GITTUF_DEV=1)
      --verbose                      enable verbose logging
```

### SEE ALSO

* [gittuf](gittuf.md)	 - A security layer for Git repositories, powered by TUF

