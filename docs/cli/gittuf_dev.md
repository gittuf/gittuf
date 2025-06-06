## gittuf dev

Developer mode commands

### Synopsis

The 'dev' command group provides advanced utilities for use during gittuf development and debugging. These commands are intended for internal or development use and are not designed to be run in production or standard repository workflows. Improper use may compromise repository security guarantees. To enable these commands, the environment variable GITTUF_DEV must be set to 1.

### Options

```
  -h, --help   help for dev
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
* [gittuf dev rsl-record](gittuf_dev_rsl-record.md)	 - Record explicit state of a Git reference in the RSL, signed with specified key (developer mode only, set GITTUF_DEV=1)

