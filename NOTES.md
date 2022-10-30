# Notes

This document covers several workflows users employ in their use of Git. This
analysis is important to understand how gittuf will change user workflows.

**AN:** A significant amount of this is a braindump still and needs revising and
clarifications. I needed to get these ideas somewhere concrete first.

## Initializing a new repository -- `git init`

This command is used to create a new Git repository. This entails creating the
`.git` directory (by default--the user can specify an alternate name) and the
directory structure within it.

gittuf will require some additional operations. Not only must it initialize a
new repository, it must initialize a TUF root. In this situation, it is assumed
that the user executing the command is one of the owners of the repository. The
owner specifies an expiry date and can add one or more public keys that map to
other functionaries of the repository. Finally, they sign it using their root
key. gittuf then writes this file to the `refs/tuf/root.ext` within the `.git`
directory or its equivalent.

**Q:** Should `init` also add the top level Targets role?

**Q:** What is the blessed copy of the repository? Do we hand that off to
`remote`?

### Edge Case 1 -- Running `init` on an existing repository

`git init` has no impact in an existing repository. However, there may be uses
to running `gittuf init` to (re-)initialize the TUF root for the repository. If
a TUF root already exists, the tool must exit with a warning and allow users to
forcefully overwrite the existing TUF root with a new one.

### Additional Thoughts

Git includes support for templates that are copied into the `.git` directory or
its equivalent. gittuf can probably utilize this functionality to initialize its
namespace. This will avoid issues with detached or renamed `.git` directories.

The initialization process must also prompt owners to create sets of users /
keys based on what refs or files each set is authorized to modify. This
information must be used to automatically create the initial delegations tree.
"Recent Writer Trust" is relevant here.

## Making a change -- `git add` and `git commit`

The most common workflow used when recording changes within a repository is as
follows:

```
git add ...
git commit ...
```

The `add` command adds one or more files specified to Git's "staging" area. The
changes in the staging area are then recorded into a commit using the `commit`
command. gittuf does not need to keep track of the changes being added to the
staging area. This is because gittuf's policies are applied with respect to
commits--files the current user does not have access to can be modified and
added to the staging area and this does not matter until the user tries to
actually commit these changes.

On the other hand, a number of additions to the Git's `commit` workflow are
necessary. First, gittuf must check if updated TUF targets metadata is
available. This is important because new policies may have been issued that
provide or revoke access to the committing user for a set of files. If the user
has modified a file they aren't authorized to, gittuf must terminate without
committing the changes. Similarly, gittuf must also validate that the changes
are in a branch the user is authorized to write to. If not, the process must
be terminated without committing the changes.

**Q:** Should gittuf download the latest set of TUF metadata from some
designated remote? How is the blessed remote managed?

**Q:** What if the user is making changes to files they're not authorized to
write to but in a separate feature branch? A repo owner with write access to a
protected branch and the files in question may choose to merge it, and that
should be considered valid? I think there isn't one right answer here, and we
should show variations of this using delegation semantics with corresponding
policies.

Second, once past verifying if the user is making an authorized change, a new
commit object must be created using default Git semantics. Git provides the
default SHA-1 identifier for this commit object. The same process used to
generate this identifier must be performed with SHA-256 to obtain that
identifier.
[Read more](#providing-sha-256-identifiers-alongside-existing-sha-1-identifiers).

Third, the corresponding targets role must be updated with both the SHA-1 and
SHA-256 hashes for the target branch. The metadata must be signed by the user's
private key. A delegation structure like that used in "Recent Writer Trust"
likely makes the most sense and the reordering described there must be performed
by gittuf.

### Edge Case 1 -- Amending an existing commit

In some cases, users can choose to amend an existing commit with new commits.
The user stages the changes they want to add to the commit and use the
`--amend` flag to edit the existing commit. In this scenario, gittuf can apply
the same workflow as when a new commit is being created--validate that the
user is authorized to modify the files and update the targets metadata with the
new commit's hashes for the corresponding ref.

### Edge Case 2 -- Rebasing a series of commits

When a commit is rebased, its history is edited. This can have significant
consequences if gittuf is unable to correctly validate the new sequence of
changes. Rebasing a series of commits essentially creates a new series of
commits that may or may not have the same changes. In fact, the commits may not
even be in the same location in the commit Merkle DAG.

During a rebase, Git's commit workflow is applied to the series of changes the
user selects. The user may choose to pause a rebase and amend commits in the
middle of a range of commits. Therefore, gittuf must make no assumptions about
the changes in a rebase based on the prior state of the series of commits. At
the formation of each commit, gittuf must apply the same series of checks as
with creating a new commit and abort appropriately.

### Edge Case 3 -- Cherry-picking a series of commits

A cherry-pick applies the changes from the selected commits into a target
branch. New commit objects are created that correspond to each commit
cherry-picked. As before, when cherry-picking each commit, the full workflow
must be applied to ensure the committer is authorized to the target branch and
to make changes to the selected files.

## Merging changes from feature branches to protected branches

This workflow shares several characteristics with that of
[adding commits](#making-a-change----git-add-and-git-commit). However, a key
difference is that a merge can place commits from an unauthorized user in a
protected branch **if** the merge was initiated by an authorized user.

First, as before, gittuf checks for new TUF targets metadata. This ensures the
merge is performed with the latest set of policies. These policies are checked
to see if the merging user is authorized to make changes to the files modified
in the commits. Similarly, the policies are checked to ensure the merging user
is indeed authorized to write to the base branch. FIXME: branch should probably
be checked first.

Second, as before, a new _merge_ commit object is created by the merging user.
The SHA-1 identifiers are mapped to their corresponding SHA-256 identifiers.
Finally, the right targets role is updated with the identifiers of the merge
commit.

## Pushing changes to blessed repository

This workflow needs to be designed carefully. Not only must metadata from
multiple clients be handled correctly (with delegations in the right order),
the server on receiving a set of changes must also handle conflict resolution
and recovery in potential attacks.

When a user invokes the push workflow to submit a set of changes from their
local copy to the remote blessed repository, they may hold changes that are
now considered unauthorized by the remote. As such, all pushes should begin with
a refresh of the client's TUF metadata to verify that the user is authorized to
push to the target branch and that the changes being pushed are in files the
user can write to. If this verification fails, the push operation should be
terminated.

If the verification passes, the local targets metadata with a record of the new
changes and the changes themselves must be submitted to the repository.

**N:** We should ensure this is atomic to avoid the metadata being out of sync
from the actual states of the repositories.

**N:** We should address situations where the user has made changes to multiple
branches locally (and the metadata reflects that), but is only pushing changes
upstream to one branch. The delegations tree should perhaps mirror the branches.
Do we still have the recent writer trust issue if we split up the metadata?

### Resolving Git conflicts

A push operation will fail if the client contains some changes that conflict
with those on the remote. In these cases, the user is prompted to fetch the
changes from the remote, resolve the conflicts, and then push again. gittuf
should be able to handle these situations using semantics already described for
[merging](#merging-changes-from-feature-branches-to-protected-branches) and
[rebasing](#edge-case-2----rebasing-a-series-of-commits) commits.

### Recovering from accidental changes and pushes

A gittuf repository is simply a Git repository with an extra set of metadata. As
such, it is quite possible for users to make changes to their copies of the
repository without using gittuf, but rather the Git command directly. This means
that the metadata they hold may be out of sync from the actual state of the
repository.

There are several scenarios here. If a user makes changes locally and tries to
push them to the blessed copy, it should be quite easy to detect and reject the
changes. A pre-receive hook on the server can be employed to ensure the client
is also sending valid metadata for the set of changes. If not, the operation
must be terminated.

**N:** We should map out the scenarios where the blessed repo has been
compromised and the hooks have been disabled. We can likely make use of
something like Rekor to point to last known policies for the repo? This is
likely a separate aspect of validation of the repo state.

When the user is prompted to sign new metadata, they should be able to use
gittuf to "catch up" to the current state of their copy of the repository. Prior
to signing new targets metadata, gittuf must validate that the changes made
compared to its previous entry are allowed by the repository's policies for the
corresponding branches and files.

**N:** How do we handle situations where history has been rewritten and gittuf's
recorded state doesn't have a clear path to the current state?

### Recovering from a developer compromise

In this scenario, one or more developer keys have been compromised and used to
sign valid metadata for malicious changes. These metadata and changes are pushed
to the blessed repository. Note that this assumes the TUF root keys are not
among those compromised.

In this scenario, the repository owners must immediately sign new metadata that
removes the compromised keys. The Git repository must be reverted to the last
known good state, and the owners must issue new targets metadata that records
this state.

### Recovering from an unsynchronized state on the blessed repository

This state occurs when the repository branches have different states than what
is recorded in its TUF metadata. The repository owners must first assess if the
repository state is malicious--an attacker was able to push malicious changes
but lacked the ability to sign metadata. If this is the case, the owners must
revert the changes to the last known good state matched in the metadata. The
specific set of circumstances that allowed attackers to push to the repository
is out of scope of gittuf but they must be appropriately handled.

If the synchronization was not the result of an attack, the commits that were
not recorded must be checked against the active delegations policy to ensure
they were valid. If yes, the owners or the appropriate developer must sign new
metadata reflecting that the changes were authorized. Appropriate measures must
again be taken, eg. the aforementioned server side hook, to avoid this situation
from recurring.

## Providing SHA-256 identifiers alongside existing SHA-1 identifiers

By default, Git uses the SHA-1 hash algorithm to calculate unique identifiers.
There is experimental support for SHA-256 identifiers, but:
1. repositories can't currently use both SHA-1 and SHA-256 identifiers, so
   converting existing repositories means the loss of development history.
2. most Git servers or forges don't support SHA-256 identifiers.

Since gittuf maintains a separate set of metadata about the Git objects in a
repository, it can also provide a mapping to SHA-256 identifiers. This requires
gittuf to maintain a SHA-256 reference to every SHA-1 identifier that exists in
a repository.

### Background: SHA-1

Git stores all its objects in a content addressed store located under
`.git/objects`. This directory contains subdirectories that act as an index to
the hashes themselves. For example, the Git object for commit
`4dcd174e182cedf597b8a84f24ea5a53dae7e1e7` is stored as
`.git/objects/4d/cd174e182cedf597b8a84f24ea5a53dae7e1e7`. The hash is
calculated across the corresponding object prior to compressing it, and it can
be recalculated as follows:

```
$ cat .git/objects/4d/cd174e182cedf597b8a84f24ea5a53dae7e1e7 | zlib-flate -uncompress | sha1sum
4dcd174e182cedf597b8a84f24ea5a53dae7e1e7  -
```

### Supporting SHA-256

There are several types of Git objects: commits, blobs, and trees. Commits
record changes made in the repository. Blobs are files in the repository while
trees map to the directory structure of the repository. Trees contain a record
of blobs and subtrees.

Git commits store a record of their one or more parent commits (creating a
Merkle DAG). Each commit also points to the specific tree object that
represents the root of the repository.

```
$ git cat-file -p db1c7b0210513a452b0b971e1912d5eb2e3ffcd0
tree 7b968da28453b323a0d3333e3be4030b870d26e4
parent 4dcd174e182cedf597b8a84f24ea5a53dae7e1e7
...
```

#### Approach 1

Now, there are several ways to calculate SHA-256 identifiers. The simplest way
is to calculate the SHA-256 hash of the commit object itself.

```
$ cat .git/objects/4d/cd174e182cedf597b8a84f24ea5a53dae7e1e7 | zlib-flate -uncompress | sha256sum
c9262d30f2dd6e50088acbfb67fa49bb3e80c30e57779551727bc89fcf02e21b  -
```

However, if a SHA-1 collision is successfully performed within the repo, this
technique has some blind spots. A collision with a commit object will be
detected as two distinct commit objects may collide in SHA-1 but almost
overwhelmingly won't in SHA-256. However, a collision in the tree object is
more dangerous. In this situation, the commit object can remain the same but
point to a malicious version of the tree. The SHA-256 identifier will not
detect this change.

#### Approach 2

A more involved way of calculating SHA-256 identifiers requires every object in
the repository with a SHA-1 object to have a SHA-256 identifier. In this
method, gittuf maintains a SHA-1 to SHA-256 mapping for every object in Git's
content addressed store. This mapping can be a simple key value dictionary.
When gittuf is invoked to calculate new identifiers, say when creating a new
commit, it must use Git's default semantics to create the object with SHA-1
identifiers. For each new object created, it must replace SHA-1 identifiers with
their SHA-256 equivalents, calculating them recursively if necessary, and then
finally calculate the SHA-256 hash. For every new object encountered, a SHA-1 to
SHA-256 entry must be added to the key value record.

Note that in this method, the new objects are not written to `.git`. Instead,
the objects continue to be stored with their SHA-1 identifiers. The only change
is the addition of the file with the key value mapping.

However, a parallel set of objects could be maintained with SHA-256 identifiers
that are symbolic links to their SHA-1 counterparts. Note that this will
probably not play well with Git's packfiles while maintaining a separate mapping
will.

**Q:** How much extra space does it take to store both versions of the objects?

An extra reason to use this technique is forward compatibility. As noted
before, Git includes experimental support for SHA-256. Here, a repository must
be initialized with the object format set to SHA-256. From then on, all object
identifiers are calculated using SHA-256 and stored in `.git/objects`. The same
data structures are maintained, except all SHA-1 identifiers are replaced with
SHA-256 identifiers. This is similar to the technique described here, meaning
that SHA-256 identifiers calculated by gittuf are the same as Git's SHA-256
identifiers. This will play well with any transition techniques provided by Git
for SHA-1 repositories to SHA-256 in future.

### Commit / Tag signing

By default, Git signs commits using a SHA-256 representation of the commit
objects. However, these commit objects contain SHA-1 references. A collision of
the tree object referenced in the commit wouldn't be caught.

As such, the verification workflow for a commit must also validate that the
objects referenced by SHA-1 hashes also have the correct SHA-256 hashes. After
they are validated, the signature can be verified using the relevant public key
to check the identify of the committer.

**Q:** Verification of SHA-256 hashes requires that the object be present as
well. How does this work when fetching new objects? Only a malicious object
that has a SHA-1 collision may be presented, meaning we don't have a reference
of the correct SHA-256 hash.

**T:** We'd have to pass around a prior calculated SHA-256 hash via the
translation mapping. However, if that must be trusted, we'd also have to ensure
it wasn't tampered with. TUF semantics can help here.
