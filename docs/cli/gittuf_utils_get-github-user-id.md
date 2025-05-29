## gittuf utils get-github-user-id

Gets the user ID of the specified GitHub user

### Synopsis

This command gets the user ID of the specified GitHub user, needed in a user's identity definition if they are to approve pull requests on GitHub and the gittuf GitHub app is used

```
gittuf utils get-github-user-id [flags]
```

### Options

```
      --github-token string   token for the GitHub API
      --github-url string     URL of the GitHub instance to query (default "https://github.com")
  -h, --help                  help for get-github-user-id
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

* [gittuf utils](gittuf_utils.md)	 - Supporting tools for gittuf's operation

