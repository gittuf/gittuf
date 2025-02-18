## gittuf dev

Developer mode commands

### Synopsis

These commands are meant to be used to aid gittuf development, and are not expected to be used during standard workflows. If used, they can undermine repository security. To proceed, set GITTUF_DEV=1.

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
* [gittuf dev populate-cache](gittuf_dev_populate-cache.md)	 - Populate persistent cache (developer mode only, set GITTUF_DEV=1)
* [gittuf dev rsl-record](gittuf_dev_rsl-record.md)	 - Record explicit state of a Git reference in the RSL, signed with specified key (developer mode only, set GITTUF_DEV=1)

