# gittuf Policy for Multiple Repositories

Last Modified: November 3, 2024

Status: Draft

Currently, gittuf must be deployed on a per-repository basis: each Git
repository has its own independent set of policy metadata. This makes it
difficult to scale gittuf when there are many repositories, as each repository’s
root of trust must be bootstrapped separately for secure verification. This is
especially a bottleneck for enterprise contexts where there may be thousands of
repositories. Therefore, in this document, we explore how multiple
gittuf-enabled repositories can share some policy metadata with an initial focus
on the root of trust.

The initial proposal is for the shared policy is expected to be limited to the
root of trust metadata. As such, each repository will have its own primary rule
file that inherits from the shared root of trust. The shared root of trust
itself is stored in a “controller repository”. Note that in order to be able to
track changes to the shared root of trust, the RSL must also be shared across
all of these repositories. This is because the RSL records the state of the
policy reference; every repository that inherits the root of trust must be aware
of changes to the root of trust. Thus, the RSL is also part of the controller
repository. TODO: discuss RSL sharding options

Each repository that "inherits" from the controller repository has a special
remote configured for the controller repository. This remote is used to update
the RSL and the policy. The remote’s location is tracked in a special
`refs/gittuf/controller` ref that isn’t synchronized with the remote. During
interactions with the remote, if a controller is set, gittuf uses the controller
remote to fetch / push rsl entries to. TODO: discuss using Git config instead

## Synchronizing Controller Contents

NOTE: This is a dump of initial ideas.

The controller repository has two references:
* `refs/gittuf/reference-state-log` -> The RSL for itself and all inheriting
  repositories
* `refs/gittuf/policy` -> Policy metadata, starting with the Root of Trust

Creating an inheriting repository:
* gittuf initialization is performed by passing in the location of the
  controller repository
* Initialization process sets up the controller repository as a special Git
  remote
* The full RSL is fetched from the controller repository to the initialized
  inheriting repository
* The controller repository’s policy reference is fetched to a temporary
  reference and is verified using the RSL (assumes expected root keys are passed
  in as well or this is TOFU
* The controller’s policy reference is NOT stored as the inheriting repository’s
  policy reference; instead, the metadata files contained in the latest policy
  state are copied over to the inheriting repository’s refs/gittuf/policy
  reference
* This is recorded in the RSL using a new entry type: an RSL import entry
    * This entry type needs to be nailed down but its structure could likely be
      similar to the reference entry type; an explicit new type would make it
      easier to distinguish between “policy import” events
    * Pros of making import event its own thing: verification could be performed
      with a repository in a self contained manner without having to communicate
      with a separate repository
    * Con: we introduce a new tree/commit ID for a policy state that can be
      difficult to map back to the original policy state; this could be
      addressed explicitly using the import RSL entry though

Updating an inheriting repository with remote changes:
* Fetch RSL entries from controller remote to local RSL reference
* Fetch controller policy reference to a temporary reference
* For each change in policy recorded in the RSL, perform consecutive state
  verification to validate root of trust
* Import _each new_ policy state and record as a distinct import event in the
  RSL

Recording a change in an inheriting repository:
* Fetch RSL entries from controller remote to local RSL reference (i.e., the
  usual workflow)
* Fetch controller policy to a temporary reference
* Import each new policy state in the RSL into local repository’s policy
  reference
* Verify new entries (using the usual workflow)
* Record entry for new change
* Push references

Making policy change in inheriting repository:
* Make a policy change to a rule file that is part of the local repository
* Add RSL entry recording the change, including the inheriting repository's
  identifier

## Notes

We probably need some notion of an inheriting RSL entry described above. If the
RSL can be sharded securely and we can guarantee that RSL shards can be kept
updated with the controller's changes, then some of the workflows described
above can be updated.

There are questions around availability: what if the controller repository is
unavailable for some reason?
