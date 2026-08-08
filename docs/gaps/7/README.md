# gittuf Guarantees for Repository Subtrees

## Metadata

* **Number:** 7
* **Title:** gittuf Guarantees for Repository Subtrees
* **Implemented:** No
* **Withdrawn/Rejected:** No
* **Sponsors:** Aditya Sirish A Yelgundhalli (adityasaky), Patrick Zielinski (patzielinski), Dennis Roellke (dns43)
* **Last Modified:** March 25, 2025

## Abstract

This GAP proposes the "propagation" pattern for gittuf repositories. The
propagation pattern defines the mechanism to take the contents of an upstream
gittuf-enabled repository and make it available in another, downstream
gittuf-enabled repository. The pattern introduces some changes to standard
gittuf workflows to ensure that subsequent changes are also propagated, and
there is a record of the states clients see of upstream or downstream
repositories.

## Specification

gittuf propagation is a mechanism to check in the contents of an upstream
gittuf-enabled repository at a specified upstream reference into a path in one
or more references in a downstream gittuf-enabled repository. Propagation
applies changes into the downstream repository when the upstream repository's
RSL indicates there's an update to the tracked reference. The contents at the
revision indicated by the upstream repository's RSL entry for the reference are
copied into the specified path in the specified downstream reference in the
downstream repository. For example, a downstream repository may track the `main`
branch of the `libfoo` repository and have it propagated into the downstream
repository's `main` branch at the location `third_party/libfoo`.

### Configuring Upstream Repositories for Propagation

Repository propagation directives are stored in the root of trust metadata.
Propagations are defined in a new top level field in the root of trust metadata.
Each propagation directive contains the following fields:

* Name: a unique name to identify the directive
* Upstream Repository: the location of the upstream repository, this must be a
  clonable URL
* Upstream Reference: the Git reference to propagate from in the upstream
  repository
* Downstream Reference: the Git reference to propagate into in the current
  repository
* Downstream Path: the tree location to propagate upstream components to in the
  current repository in the specified downstream reference

### Propagation Workflow

The propagation workflow is executed for every propagation directive declared in
the repository. Thus, for some directive:

1. Identify upstream repository `U`
1. Temporarily clone and fetch `U`, including its RSL
1. If `U` does not have an RSL, abort with an error that the upstream repository
   is not gittuf enabled
1. Identify upstream reference `Ru`
1. Find the latest unskipped RSL entry `Eu` for `Ru` in the upstream
   repository's RSL (this may be a reference entry or a propagation entry)
1. Identify the downstream reference `Rd`
1. Find the latest unskipped RSL entry `Ed` for `Rd` in the downstream
   repository's RSL (this may be a reference entry or a propagation entry)
1. Identify the path to propagate contents to `P`
1. Find the tree ID of `P` in the root tree of `Rd` at the target specified by
   `Ed`
1. Find the root tree ID of `Ru` at the target specified by `Eu`
1. If the tree IDs are the same, return without errors as there is nothing to
   propagate
1. Copy the contents of the upstream repository's root tree into `P` in `Rd`,
   create a new commit and set as tip of `Rd`
1. Create a new propagation entry in the downstream repository's RSL identifying
   `Rd` as the reference, the new commit ID as the target, `U` as the upstream
   repository, and `Eu`'s ID as the upstream entry ID

### Recording Propagation in the RSL

For each reference, the propagation workflow records a new entry in the RSL
indicating that the changes were propagated over. The propagation entry has all
the fields as a regular reference entry. It also identifies the entry of the
upstream repository that was propagated over. The propagation entry has the
following structure:

```
RSL Propagation Entry

ref: <local ref name>
targetID: <local target ID>
upstreamRepository: <upstream repository location>
upstreamEntryID: <upstream RSL reference entry ID>
number: <number>
```

### Impact of Propagation on Existing gittuf Workflows

When a downstream repository is updated, i.e., a new RSL entry is created, the
gittuf client must also check the upstream repository's RSL for any updates. If
the upstream RSL has changes that must be propagated over, the propagation must
be executed before the downstream repository's RSL is updated with any other
changes. This ensures that changes are propagated as quickly as possible to
downstream repositories, and are applied to the RSL prior to any other
downstream changes.

## Motivation

Frequently, a Git repository also needs to include within it another repository
which is developed with its own history, for example, a project `foo` wants to
use a library `bar`. While Git has features built in to enable consuming a Git
repository within another, these lack the guarantees provided by gittuf.

## Reasoning

This section discusses how gittuf's propagation pattern compares to existing
solutions.

### Propagation vs Git Submodules

The Git Submodules feature allows a repository to be stored inside another
repository at a specified path. Each submodule is tracked in a `.gitmodules`
file in the branch where a submodule is to be added. This file can track
multiple submodule entries. Each submodule entry tracks the upstream
repository's location as well as the path into which the upstream repository's
contents must be made available in the worktree.

For every submodule, Git tracks the commit that must be checked out at that
worktree location. The upstream repository's contents are not included in the
repository. A developer that wants to work with a submodule must indicate to
their Git client that the submodule contents must be fetched and checked out in
the relevant locations in their worktree. In contrast, gittuf's propagation
maintains a copy of the upstream repository's contents in the consuming
repository. This mitigates availability issues with the upstream repository; it
only needs to be available at the time of propagation for the contents at that
revision to be available to all developers of the downstream repository. This
also means that developers do not have to perform additional steps to fetch the
upstream repository when changes are made. For example, Alice may update a
submodule to point to a newer commit. A regular fetch by Bob will update the
`.gitmodules` file but will not actually update the submodule's contents checked
out unless explicitly specified or configured to do so. On the other hand, with
a gittuf propagation, Bob's fetch will update the directory as well because it
is tracked as a regular Git tree whose contents have changed.

Additionally, while a Git submodule can be configured to track a particular
upstream reference (i.e., it's not limited to the default branch or the upstream
HEAD), updates to that branch in the upstream repository are not automatically
applied into the downstream repository's submodule reference. gittuf's
propagation workflow is executed in the background to check for and apply
upstream repository changes. This is an advantage when the intent is to
continuously track an upstream repository's branch (e.g., the main branch). On
the other hand, the Git submodule may go stale without regular interventions by
the developers.

Further, while a submodule may be configured to track a particular branch in the
`.gitmodules` file, in reality a submodule is pinned to a particular commit. The
branch information is only used when a developer wants to update a submodule's
contents with its upstream source. However, a developer could set the submodule
to any other commit that's not part of the branch specified in `.gitmodules`,
and another developer's client will use that commit unless the developer in
question explicitly chooses to update the submodule against its remote. For
example, Alice may set a submodule in a repository to a commit on the `unstable`
branch when it should be tracking the `main` branch. When Bob fetches the
repository and updates his worktree's submodules, his client will set the
submodule to the commit chosen by Alice in the `unstable` branch. Bob can ensure
the submodule matches the _current tip_ of the `main` branch, but he cannot
immediately determine if the commit Alice set the submodule to was never in the
`main` branch or was just an older commit in the `main` branch without manual
inspection. This is made worse by how [certain forges handle commit
availability](https://trufflesecurity.com/blog/anyone-can-access-deleted-and-private-repo-data-github),
as commits that only exist in a fork of the repository may be used as though
they belong to the primary repository.

Finally, a Git submodule keeps the commit history for the upstream repository
separate, and is available when checked out to the developer. On the other hand,
with gittuf propagation, there is no upstream history tracking.

### Propagation vs Git Subtree

Git Subtree is very similar to gittuf's propagation pattern. A specified
repository is added as a subtree in the downstream repository. Unlike
submodules, the developer does not need to do anything special to initialize or
otherwise make available the remote repository's contents in the local worktree.

The subtree feature preserves the upstream repository's commit log by default,
though typically the `squash` approach is recommended which compresses the
entire upstream history into a single commit. When a subtree is updated with the
remote's contents, a new commit is created in the branch in the downstream
repository that identifies the upstream repository's commit ID that was copied
as a subtree as well as the downstream repository's branch tip. This new commit
is a specially crafted merge commit between the histories of the upstream
repository's commit (possibly squashed) and the downstream repository's tip for
the branch where the subtree is added.

A subtree, once added, is not automatically updated. A developer must choose to
update explicitly. Additionally, there is no tracking of the actual reference in
the upstream repository pulled in as a subtree. While a reference can be
provided to add or update the subtree, if the intent is for the subtree to track
a reference, there is no validation that the upstream commit pulled in is part
of the reference that must be tracked, and is the current tip of the upstream
reference at that point in time.

Note: Git subtree is not part of Git tooling, instead this lives in the
`contrib` folder of the Git source code. As a consequence, there's no guarantee
that this is available with a developer's pre-installed Git client that gittuf
uses.

## Backwards Compatibility

This GAP introduces a new RSL entry type to record propagations. Older clients
that are unaware of the propagation type will mishandle such entries, and
therefore clients will need to be updated to support this.

## Security

### Handling Revoked Propagated Changes

If the upstream entry is revoked after being propagated to a downstream
repository, the next propagation check in the downstream repository identifies
that the latest unrevoked upstream entry is different, and thus the revoked
changes will be replaced. In other words, a special revocation flow is not
necessary for propagation.

### Handling Propagation into Protected Namespace in Downstream Repository

TODO

## Blockers to Implementing

Before this GAP is considered to be implemented, the following items must be
addressed. Until then, any implementation of the GAP is a prototype.

1. Should a gittuf client add "witness" entries as well? A gittuf client says I
   checked upstream and saw RSL entry X, nothing to propagate. Next time, a
   client needn't check beyond entry X. However, we must be careful with how
   this is verified as a malicious client could skip propagating and then say
   nothing to propagate.
2. How should propagation checks be performed to avoid adding substantial
   overhead due to lookups? For instance, checking on every push may be too
   often when pushes happen frequently.
3. How must propagations to a branch be handled when a push is made to update
   the same branch?
4. What must be checked in the upsteam repository by a client propagating from
   upstream repository into downstream repository? Full verification of the ref
   against upstream gittuf policy?
5. How must a client handle unavailability of an upstream repository?
6. What if client must propagate changes from upstream to `foo/` but the
   repository has a `file:foo/*` policy in place? Like the recovery flow, can
   anyone propagate? Should adding propagation workflow ensure downstream path
   is not protected by a file rule?
7. What's the UX for upstream repository history in downstream repository?

## References

* [Git submodule](https://git-scm.com/docs/git-submodule)
* [Git subtree](https://manpages.debian.org/testing/git-man/git-subtree.1.en.html)
