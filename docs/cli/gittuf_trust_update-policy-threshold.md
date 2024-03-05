## gittuf trust update-policy-threshold

Update Policy threshold in the gittuf root of trust (developer mode only, set GITTUF_DEV=1)

### Synopsis

This command allows users to update the threshold of valid signatures required for the policy.

DO NOT USE until policy-staging is working, so that multiple developers can sequentially sign the policy metadata.
Until then, this command is available in developer mode only, set GITTUF_DEV=1 to use.

```
gittuf trust update-policy-threshold [flags]
```

### Options

```
  -h, --help            help for update-policy-threshold
      --threshold int   threshold of valid signatures required for main policy (default -1)
```

### Options inherited from parent commands

```
      --profile                      enable CPU and memory profiling
      --profile-CPU-file string      file to store CPU profile (default "cpu.prof")
      --profile-memory-file string   file to store memory profile (default "memory.prof")
  -k, --signing-key string           signing key to use to sign root of trust
      --verbose                      enable verbose logging
```

### SEE ALSO

* [gittuf trust](gittuf_trust.md)	 - Tools for gittuf's root of trust

