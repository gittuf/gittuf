## gittuf rsl record

Record latest state of a Git reference (e.g., 'main') in the RSL

### Synopsis

The 'record' command records the latest state of a Git reference in the repository's RSL. The argument must be a valid Git reference (such as 'main', 'HEAD', or a tag name). For example: 'gittuf rsl record --local-only main'. This command is used to capture and track changes to references over time for auditing and consistency.

```
gittuf rsl record [flags]
```

### Options

```
      --dst-ref string         name of destination reference, if it differs from source reference
  -h, --help                   help for record
      --local-only             local only
      --remote-name string     remote name
      --skip-duplicate-check   skip check to see if latest entry for reference has same target
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

