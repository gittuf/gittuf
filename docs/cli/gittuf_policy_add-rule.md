## gittuf policy add-rule

Add a new rule to a policy file

### Synopsis

The 'add-rule' command adds a new rule to a gittuf policy file. Each rule contains a name ('--rule-name') for the rule, one or more principals ('--authorize') who are allowed to sign within the scope of the rule, a set of rule patterns ('--rule-pattern') defining the namespaces or paths the rule governs, and a signature threshold ('--threshold'), which is the minimum number of valid signatures required to satisfy the rule. Principals can be specified by their principal IDs. By default, the main policy file (targets) is used, which can be overridden with the '--policy-name' flag.

```
gittuf policy add-rule [flags]
```

### Options

```
      --authorize stringArray      authorize the principal IDs for the rule
  -h, --help                       help for add-rule
      --policy-name string         name of policy file to add rule to (default "targets")
      --rule-name string           name of rule
      --rule-pattern stringArray   patterns used to identify namespaces rule applies to
      --threshold int              threshold of required valid signatures (default 1)
```

### Options inherited from parent commands

```
      --create-rsl-entry             create RSL entry for policy change immediately (note: the RSL will not be synced with the remote)
      --no-color                     turn off colored output
      --profile                      enable CPU and memory profiling
      --profile-CPU-file string      file to store CPU profile (default "cpu.prof")
      --profile-memory-file string   file to store memory profile (default "memory.prof")
  -k, --signing-key string           signing key to use to sign root of trust (path to SSH key, "fulcio:" for Sigstore)
      --verbose                      enable verbose logging
```

### SEE ALSO

* [gittuf policy](gittuf_policy.md)	 - Tools to manage gittuf policies

