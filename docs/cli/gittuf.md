## gittuf

A security layer for Git repositories, powered by TUF

### Options

```
  -h, --help                         help for gittuf
      --profile                      enable CPU and memory profiling
      --profile-CPU-file string      file to store CPU profile (default "cpu.prof")
      --profile-memory-file string   file to store memory profile (default "memory.prof")
      --use-git-binary               use Git binary for some operations (developer mode only, set GITTUF_DEV=1)
      --verbose                      enable verbose logging
```

### SEE ALSO

* [gittuf add-hooks](gittuf_add-hooks.md)	 - Add git hooks that automatically create and sync RSL
* [gittuf clone](gittuf_clone.md)	 - Clone repository and its gittuf references
* [gittuf dev](gittuf_dev.md)	 - Developer mode commands
* [gittuf policy](gittuf_policy.md)	 - Tools to manage gittuf policies
* [gittuf rsl](gittuf_rsl.md)	 - Tools to manage the repository's reference state log
* [gittuf trust](gittuf_trust.md)	 - Tools for gittuf's root of trust
* [gittuf verify-commit](gittuf_verify-commit.md)	 - Verify commit signatures using gittuf metadata
* [gittuf verify-ref](gittuf_verify-ref.md)	 - Tools for verifying gittuf policies
* [gittuf verify-tag](gittuf_verify-tag.md)	 - Verify tag signatures using gittuf metadata
* [gittuf version](gittuf_version.md)	 - Version of gittuf

