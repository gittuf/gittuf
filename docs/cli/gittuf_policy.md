## gittuf policy

Tools to manage gittuf policies

### Options

```
  -h, --help                 help for policy
  -k, --signing-key string   signing key to use to sign policy file
```

### Options inherited from parent commands

```
      --profile                      enable CPU and memory profiling
      --profile-CPU-file string      file to store CPU profile (default "cpu.prof")
      --profile-memory-file string   file to store memory profile (default "memory.prof")
      --verbose                      enable verbose logging
```

### SEE ALSO

* [gittuf](gittuf.md)	 - A security layer for Git repositories, powered by TUF
* [gittuf policy add-key](gittuf_policy_add-key.md)	 - Add a trusted key to a policy file
* [gittuf policy add-person](gittuf_policy_add-person.md)	 - Add a trusted person to a policy file (requires developer mode and v0.2 policy metadata to be enabled, set GITTUF_DEV=1 and GITTUF_ALLOW_V02_POLICY=1)
* [gittuf policy add-rule](gittuf_policy_add-rule.md)	 - Add a new rule to a policy file
* [gittuf policy apply](gittuf_policy_apply.md)	 - Validate and apply changes from policy-staging to policy
* [gittuf policy init](gittuf_policy_init.md)	 - Initialize policy file
* [gittuf policy list-principals](gittuf_policy_list-principals.md)	 - List principals for the current policy in the specified rule file
* [gittuf policy list-rules](gittuf_policy_list-rules.md)	 - List rules for the current state
* [gittuf policy remote](gittuf_policy_remote.md)	 - Tools for managing remote policies
* [gittuf policy remove-rule](gittuf_policy_remove-rule.md)	 - Remove rule from a policy file
* [gittuf policy reorder-rules](gittuf_policy_reorder-rules.md)	 - Reorder rules in the specified policy file
* [gittuf policy sign](gittuf_policy_sign.md)	 - Sign policy file
* [gittuf policy tui](gittuf_policy_tui.md)	 - Start the TUI for managing policies
* [gittuf policy update-rule](gittuf_policy_update-rule.md)	 - Update an existing rule in a policy file

