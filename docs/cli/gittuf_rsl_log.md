## gittuf rsl log

Display the repository's Reference State Log

### Synopsis

The 'log' command displays the repository's RSL. It is used to view the history of reference state changes and inspect prior entries in the RSL.

```
gittuf rsl log [flags]
```

### Options

```
  -h, --help              help for log
      --ref stringArray   only display RSL entries for the specified references
```

### Options inherited from parent commands

```
      --no-color                     turn off colored output
      --profile                      enable CPU and memory profiling
      --profile-CPU-file string      file to store CPU profile (default "cpu.prof")
      --profile-memory-file string   file to store memory profile (default "memory.prof")
      --storer-trace                 report Git storage backend call counts, timings and git fork counts on exit
      --storer-trace-file string     file to store the Git storage backend trace (default "storer.trace")
      --verbose                      enable verbose logging
```

### SEE ALSO

* [gittuf rsl](gittuf_rsl.md)	 - Tools to manage the repository's reference state log

