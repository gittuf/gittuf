## gittuf trust

Tools for gittuf's root of trust

### Synopsis

The 'trust' command provides tools to manage gittuf's root of trust, including subcommands to initialize trust, add/remove keys and rules, configure GitHub App approvals, sign commits, and manage policy settings enforced through gittuf's RSL framework.

### Options

```
      --create-rsl-entry     create RSL entry for policy change immediately (note: the RSL will not be synced with the remote)
  -h, --help                 help for trust
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
* [gittuf trust add-controller-repository](gittuf_trust_add-controller-repository.md)	 - Add a controller repository
* [gittuf trust add-github-app](gittuf_trust_add-github-app.md)	 - Add GitHub app to gittuf root of trust
* [gittuf trust add-global-rule](gittuf_trust_add-global-rule.md)	 - Add a new global rule to root of trust
* [gittuf trust add-hook](gittuf_trust_add-hook.md)	 - Add a script to be run as a gittuf hook, specify when and where to run it (developer mode only, set GITTUF_DEV=1)
* [gittuf trust add-network-repository](gittuf_trust_add-network-repository.md)	 - Add a network repository
* [gittuf trust add-policy-key](gittuf_trust_add-policy-key.md)	 - Add Policy key to gittuf root of trust
* [gittuf trust add-propagation-directive](gittuf_trust_add-propagation-directive.md)	 - Add propagation directive into gittuf root of trust
* [gittuf trust add-root-key](gittuf_trust_add-root-key.md)	 - Add Root key to gittuf root of trust
* [gittuf trust apply](gittuf_trust_apply.md)	 - Validate and apply changes from policy-staging to policy
* [gittuf trust disable-github-app-approvals](gittuf_trust_disable-github-app-approvals.md)	 - Mark GitHub app approvals as untrusted henceforth
* [gittuf trust enable-github-app-approvals](gittuf_trust_enable-github-app-approvals.md)	 - Mark GitHub app approvals as trusted henceforth
* [gittuf trust get-github-app-approvals-status](gittuf_trust_get-github-app-approvals-status.md)	 - Get whether GitHub App approvals are trusted
* [gittuf trust get-github-app-key](gittuf_trust_get-github-app-key.md)	 - Get the current defined keys for GitHub Apps
* [gittuf trust get-policy-key](gittuf_trust_get-policy-key.md)	 - Get the current policy key
* [gittuf trust get-policy-threshold](gittuf_trust_get-policy-threshold.md)	 - List the currently defined global rules for the root of trust
* [gittuf trust get-repository-location](gittuf_trust_get-repository-location.md)	 - Get the current repository location
* [gittuf trust get-root-threshold](gittuf_trust_get-root-threshold.md)	 - Get the current root threshold
* [gittuf trust init](gittuf_trust_init.md)	 - Initialize gittuf root of trust for repository
* [gittuf trust inspect-root](gittuf_trust_inspect-root.md)	 - Inspect root metadata
* [gittuf trust list-global-rules](gittuf_trust_list-global-rules.md)	 - List global rules for the current state
* [gittuf trust list-hooks](gittuf_trust_list-hooks.md)	 - List gittuf hooks for the current policy state
* [gittuf trust make-controller](gittuf_trust_make-controller.md)	 - Make current repository a controller
* [gittuf trust remote](gittuf_trust_remote.md)	 - Tools for managing remote policies
* [gittuf trust remove-github-app](gittuf_trust_remove-github-app.md)	 - Remove GitHub app from gittuf root of trust
* [gittuf trust remove-global-rule](gittuf_trust_remove-global-rule.md)	 - Remove a global rule from root of trust
* [gittuf trust remove-hook](gittuf_trust_remove-hook.md)	 - Remove a gittuf hook specified in the policy (developer mode only, set GITTUF_DEV=1)
* [gittuf trust remove-policy-key](gittuf_trust_remove-policy-key.md)	 - Remove Policy key from gittuf root of trust
* [gittuf trust remove-propagation-directive](gittuf_trust_remove-propagation-directive.md)	 - Remove propagation directive from gittuf root of trust
* [gittuf trust remove-root-key](gittuf_trust_remove-root-key.md)	 - Remove Root key from gittuf root of trust
* [gittuf trust set-repository-location](gittuf_trust_set-repository-location.md)	 - Set repository location
* [gittuf trust sign](gittuf_trust_sign.md)	 - Sign root of trust
* [gittuf trust stage](gittuf_trust_stage.md)	 - Stage and push local policy-staging changes to remote repository
* [gittuf trust update-global-rule](gittuf_trust_update-global-rule.md)	 - Update an existing global rule in the root of trust
* [gittuf trust update-hook](gittuf_trust_update-hook.md)	 - Modify the parameters of an existing gittuf hook (developer mode only, set GITTUF_DEV=1)
* [gittuf trust update-policy-threshold](gittuf_trust_update-policy-threshold.md)	 - Update Policy threshold in the gittuf root of trust
* [gittuf trust update-propagation-directive](gittuf_trust_update-propagation-directive.md)	 - Update propagation directive in the root of trust (developer mode only, set GITTUF_DEV=1)
* [gittuf trust update-root-threshold](gittuf_trust_update-root-threshold.md)	 - Update Root threshold in the gittuf root of trust

