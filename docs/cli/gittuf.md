## gittuf

A security layer for Git repositories, powered by TUF

### Synopsis

gittuf is a security layer for Git repositories, powered by TUF. The CLI provides commands to manage gittuf on the repository, including trust management, policy enforcement, signing, verification, and synchronization.

### Options

```
  -h, --help                         help for gittuf
      --no-color                     turn off colored output
      --profile                      enable CPU and memory profiling
      --profile-CPU-file string      file to store CPU profile (default "cpu.prof")
      --profile-memory-file string   file to store memory profile (default "memory.prof")
      --verbose                      enable verbose logging
```

### SEE ALSO

* [gittuf add-hooks](gittuf_add-hooks.md)	 - Add git hooks that automatically create and sync RSL
* [gittuf attest](gittuf_attest.md)	 - Tools for attesting to code contributions
* [gittuf cache](gittuf_cache.md)	 - Manage gittuf's caching functionality
* [gittuf clone](gittuf_clone.md)	 - Clone repository and its gittuf references
* [gittuf dev](gittuf_dev.md)	 - Developer mode commands
* [gittuf policy](gittuf_policy.md)	 - Tools to manage gittuf policies
* [gittuf rsl](gittuf_rsl.md)	 - Tools to manage the repository's reference state log
* [gittuf sync](gittuf_sync.md)	 - Synchronize local references with remote references based on RSL
* [gittuf trust](gittuf_trust.md)	 - Tools for gittuf's root of trust
* [gittuf verify-mergeable](gittuf_verify-mergeable.md)	 - Tools for verifying mergeability using gittuf policies
* [gittuf verify-network](gittuf_verify-network.md)	 - Verify state of network repositories
* [gittuf verify-ref](gittuf_verify-ref.md)	 - Tools for verifying gittuf policies
* [gittuf version](gittuf_version.md)	 - Version of gittuf

