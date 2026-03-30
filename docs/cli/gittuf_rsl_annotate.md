## gittuf rsl annotate

Annotate prior RSL entries

### Synopsis

The 'annotate' command adds annotations to prior RSL entries in the repository's RSL. It is used to add a message to an entry for additional context or mark an entry to be skipped, in cases where RSL recovery or reconciliation is needed.

```
gittuf rsl annotate [flags]
```

### Options

```
  -h, --help                 help for annotate
      --local-only           local only
  -m, --message string       annotation message
      --remote-name string   remote name
  -s, --skip                 mark annotated entries as to be skipped
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

* [gittuf rsl](gittuf_rsl.md)	 - Tools to manage the repository's reference state log

