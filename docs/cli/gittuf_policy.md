## gittuf policy

Tools to manage gittuf policies

### Synopsis

The 'policy' command provides a suite of tools for managing gittuf policy configurations. This command serves as a parent for several subcommands that allow users to initialize policy, add or remove principals, view or reorder existing rules and principals, apply, stage, or discard trust policy changes, or interact with policies through a terminal UI.

### Options

```
      --create-rsl-entry     create RSL entry for policy change immediately (note: the RSL will not be synced with the remote)
  -h, --help                 help for policy
  -k, --signing-key string   signing key to use to sign root of trust (path to SSH key, "fulcio:" for Sigstore)
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
* [gittuf policy add-key](gittuf_policy_add-key.md)	 - Add a trusted key to a policy file
* [gittuf policy add-person](gittuf_policy_add-person.md)	 - Add a trusted person to a policy file
* [gittuf policy add-rule](gittuf_policy_add-rule.md)	 - Add a new rule to a policy file
* [gittuf policy apply](gittuf_policy_apply.md)	 - Validate and apply changes from policy-staging to policy
* [gittuf policy discard](gittuf_policy_discard.md)	 - Discard the currently staged changes to policy
* [gittuf policy init](gittuf_policy_init.md)	 - Initialize policy file
* [gittuf policy list-principals](gittuf_policy_list-principals.md)	 - List principals for the current policy in the specified rule file
* [gittuf policy list-rules](gittuf_policy_list-rules.md)	 - List rules for the current state
* [gittuf policy remote](gittuf_policy_remote.md)	 - Tools for managing remote policies
* [gittuf policy remove-key](gittuf_policy_remove-key.md)	 - Remove a key from a policy file
* [gittuf policy remove-person](gittuf_policy_remove-person.md)	 - Remove a person from a policy file
* [gittuf policy remove-rule](gittuf_policy_remove-rule.md)	 - Remove rule from a policy file
* [gittuf policy reorder-rules](gittuf_policy_reorder-rules.md)	 - Reorder rules in the specified policy file
* [gittuf policy sign](gittuf_policy_sign.md)	 - Sign policy file
* [gittuf policy stage](gittuf_policy_stage.md)	 - Stage and push local policy-staging changes to remote repository
* [gittuf policy tui](gittuf_policy_tui.md)	 - Start the TUI for managing policies
* [gittuf policy update-rule](gittuf_policy_update-rule.md)	 - Update an existing rule in a policy file

