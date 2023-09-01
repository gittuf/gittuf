# gittuf Specification

Last Modified: July 11, 2023

Version: 0.1.0-draft

## Introduction

This document describes gittuf, a security layer for Git repositories. gittuf
applies several key properties part of the
[The Update Framework (TUF)](https://theupdateframework.io/) such as
delegations, secure key distribution, key revocation, trust rotation, read /
write access control, and namespaces to Git repositories. This enables owners of
the repositories to distribute (and revoke) contributor signing keys and define
policies about which contributors can make changes to some namespaces within the
repository. gittuf also protects against reference state attacks by extending
the Reference State Log design which was originally described in an
[academic paper](https://www.usenix.org/conference/usenixsecurity16/technical-sessions/presentation/torres-arias).
Finally, gittuf can be used as a foundation to build other desirable features
such as cryptographic algorithm agility, the ability to store secrets, storing
in-toto attestations pertaining to the repository, and more.

This document is scoped to describing how TUF's access control policies are
applied to Git repositories. It contains the corresponding workflows for
developers and their gittuf clients. Note that gittuf is designed in a manner
that enables other security features. These descriptions will be in standalone
specifications alongside this one, and will describe modifications or extensions
to the "default" workflows in this document.

## Definitions

The gittuf specification uses several terms or phrases in specific ways
throughout the document. These are defined here.

### Git Reference (Ref)

A Git reference is a "simple name" that typically points to a particular Git
commit. Generally, development in Git repositories are centered in one or more
refs, and they're updated as commits are added to the ref under development. By
default, Git defines two of refs: branches (heads) and tags. Git allows for the
creation of other arbitrary refs that users can store other information as long
as they are formatted using Git's object types.

### Actors

In the context of a Git repository, an actor is any user who contributes changes
to the repository. This may be to any file tracked by the repository in any Git
ref. In gittuf, actors are identified by the signing keys they use when
contributing to the repository. A policy that grants an actor the ability to
make a certain change in fact grants it to the holder of their signing key.
Verification of any action performed in the repository depends, among other
factors, on the successful verification of the action's signature using the
expected actor's public or verification key.

### State

State describes the expected values for tracked refs of the repository. It is
identified by the tip or last entry of the
[reference state log](#reference-state-log-rsl). Note that when inspecting
changes to the state of the repository, a workflow may only consider state
updates relevant to a particular ref.

## gittuf

To begin with, gittuf carves out a namespace for itself within the repository.
All gittuf-specific metadata and information are tracked in a separate Git ref,
`refs/gittuf`.

### Reference State Log (RSL)

Note: This document presents only a summary of the academic paper and a
description of gittuf's implementation of RSL. A full read of the paper is
recommended.

The Reference State Log contains a series of entries that each describe some
change to a Git ref. Each entry contains the ref being updated, the new location
it points to, and a hash of the parent RSL entry. The entry is signed by the
actor making the change to the ref.

Given that each entry in effect points to its parent entry using its hash, an
RSL is a hash chain. gittuf's implementation of the RSL uses Git's underlying
Merkle graph. Generally, gittuf is designed to ensure the RSL is linear but a
privileged attacker may be able to cause the RSL to branch, resulting in a
forking attack.

The RSL is tracked at `refs/gittuf/reference-state-log`, and is implemented as a
distinct commit graph. Each commit corresponds to one entry in the RSL, and
standard Git signing mechanisms are employed for the actor's signature on the
RSL entry. The latest entry is identified using the tip of the RSL Git ref.

Note that the RSL and liveness of the repository in Git remove the need for some
traditional TUF roles. As the RSL records changes to other Git refs in the
repository, it incorporates TUF's
[snapshot role](https://theupdateframework.github.io/specification/latest/#snapshot)
properties. At present, gittuf does not include an equivalent to TUF's
[timestamp role](https://theupdateframework.github.io/specification/latest/#timestamp)
to guarantee the freshness of the RSL. This is because the timestamp role in the
context of gittuf at most provides a non-repudiation guarantee for each claim of
the RSL's tip. The use of an online timestamp does not guarantee that actors
will receive the correct RSL tip. This may evolve in future versions of the
gittuf specification.

#### Normal RSL Entries

These entries are the standard variety described above. They contain the name of
the reference they apply to and a commit ID. As such, they have the following
structure.

```
RSL Entry

ref: <ref name>
commit: <commit ID>
```

#### RSL Annotation Entries

Apart from regular entries, the RSL can include annotations that apply to prior
RSL entries. Annotations can be used to add more information as a message about
a prior entry, or to _explicitly_ mark one or more entries as ones to be
skipped. This semantic is necessary when accidental or possibly malicious RSL
entries are recorded. Since the RSL history cannot be overwritten, an annotation
entry must be used to communicate to gittuf clients to skip the corresponding
entries. Annotations have the following schema.

```
RSL Annotation

entryID: <RSL entry ID 1>
entryID: <RSL entry ID 2>
...
skip: <true/false>
-----BEGIN MESSAGE-----
<message>
------END MESSAGE------
```

#### Example Entries

TODO: Add example entries with all commit information. Create a couple of
regular entries and annotations, paste the outputs of `git cat-file -p <ID>`
here.

### Actor Access Control Policies

Note: This section assumes some prior knowledge of the TUF specification.

There are several aspects to how defining the access privileges an actor has.
First, actors must be established in the repository unambiguously, and gittuf
uses TUF's mechanisms to associate actors with their signing keys. TUF metadata
distributes the public keys of all the actors in the repository and if a key is
compromised, new metadata is issued to revoke its trust.

Second, TUF allows for defining _namespaces_ for the repository. TUF's notion of
namespaces aligns with Git's, and TUF namespaces can be used to reason about
both Git refs and files tracked within the repository. Namespaces are combined
with TUF's _delegations_ to define sets of actors who are authorized to make
changes to some namespace. As such, the owner of the repository can use gittuf
to define actors representing other contributors to the repository, and delegate
to them only the necessary authority to make changes to different namespaces of
the repository.

Policies for gittuf access are defined using a subset of TUF roles. The owners
of the repository hold the keys used to sign the Root role that delegates trust
to the other roles. The top level Targets role and any Targets roles it
delegates to contain restrictions on protected namespaces. The specifics of the
delegation structure vary from repository to repository as each will have its
own constraints.

A typical TUF delegation connects two TUF Targets roles. Therefore, delegations
can be represented as a directed graph where each node is a Targets role, and
each edge connects the delegating role to a delegatee role for some specified
namespace. When verifying or fetching a target, the graph is traversed using the
namespaces that match the target until a Targets entry is found for it. The
Targets entry contains, among other information, the hashes and length of the
target. gittuf applies this namespaced delegations graph traversal to Git and
also incorporate RSLs and Git's implicit change tracking mechanisms.

In gittuf, the delegations graph is similarly traversed, except that it
explicitly does not expect any Targets metadata to contain a target entry.
Instead, the delegation mechanism is used to identify the set of keys authorized
to sign the target such as an RSL entry or commit being verified. Therefore, the
delegation graph is used to decide which keys git actions should trust, but no
targets entries are used.  Any key which delegated trust up to this part of the 
namespace (including the last delegation), is trusted to sign the git actions.

![delegation_example_2](https://github.com/gittuf/gittuf/assets/14241779/963bba32-6c34-4211-80e9-87e0d6fe8836)
In this example, the repository administrator grants write permissions to Carol 
for the main branch, to Alice for the alice-dev branch, and to Bob for the /tests folder
(under any of the existing branches).

This mechanism is employed when verifying both RSL entries for Git ref updates
_and_ when verifying the commits introduced between two ref updates. The latter
option allows for defining policies to files and directories tracked by the
repository. It also enables repository owners to define closed sets of
developers authorized to make changes to the repository. Note that gittuf does
not by default use Git commit metadata to identify the actor who created it as
that may be trivially spoofed.

Another difference between standard TUF policies and those used by gittuf is a
more fundamental difference in expectations of the policies. Typical TUF
deployments are explicit about the artifacts they are distributing. Any artifact
not listed in TUF metadata is rejected. In gittuf, policies are written only to
express _restrictions_. As such, when verifying changes to unprotected
namespaces, gittuf must allow any key to sign for these changes. This means that
after all explicit policies (expressed as delegations) are processed, and none
apply to the namespace being verified, an implicit `allow-rule` is applied,
allowing verification to succeed.

In summary, a repository secured by gittuf stores the Root role and one or more
Targets roles. Further, it embeds the public keys used to verify the Root role's
signatures, the veracity of which are established out of band. The metadata and
the public keys are stored as Git blobs and updates to them are tracked through
a standalone Git commit graph. This is tracked at `refs/gittuf/policy`. The RSL
MUST track the state of this reference so that the policy namespace is protected
from reference state attacks. Further, RSL entries are used to identify
historical policy states that may apply to older changes.

## Example

Consider project `foo`'s Git repository maintained by Alice and Bob. Alice and
Bob are the only actors authorized to update the state of the main branch. This
is accomplished by defining a TUF delegation to Alice and Bob's keys for the
namespace corresponding to the main branch. All changes to the main branch's
state MUST have a corresponding entry in the repository's RSL signed by either
Alice or Bob.

Further, `foo` has another contributor, Clara, who does not have maintainer
privileges. This means that Clara is free to make changes to other Git branches
but only Alice or Bob may merge Clara's changes from other unprotected branches
into the main branch.

Over time, `foo` grows to incorporate several subprojects with other
contributors Dave and Ella. Alice and Bob take the decision to reorganize the
repository into a monorepo containing two projects, `bar` and `baz`. Clara and
Dave work exclusively on bar and Ella works on baz with Bob. In this situation,
Alice and Bob retain their privileges to merge changes to the main branch.
Further, they set up delegations for each subproject's path within the
repository. Clara and Dave are only authorized to work on files within `bar/*`
and Ella is restricted to `baz/*`. As Bob is a maintainer of foo, he is not
restricted to working only on `baz/*`.

## Actor Workflows

gittuf does not modify the underlying Git implementation itself. For the most
part, developers can continue using their usual Git workflows and add some
gittuf specific invocations to update the RSL and sync gittuf namespaces.

### Managing gittuf root of trust

The gittuf root of trust is a TUF Root stored in the gittuf policy namespace.
The keys used to sign the root role are expected to be securely managed by the
owners of the repository. TODO: Discuss detached roots, and root specific
protections for the policy namespace.

The root of trust is responsible for managing the root of gittuf policies. Each
gittuf policy file is a TUF Targets role. The top level Targets role's keys are
managed in the root of trust. All other policy files are delegated to directly
or indirectly by the top level Targets role.

```bash
$ gittuf trust init
$ gittuf trust add-policy-key
$ gittuf trust remove-policy-key
```

Note: the commands listed here are examples and not exhaustive. Please refer to
gittuf's help documentation for more specific information about gittuf's usage.

### Managing gittuf policies

Developers can initialize a policy file if it does not already exist by
specifying its name. Further, they must present its signing keys. The policy
file will only be initialized if the presented keys are authorized for the
policy. That is, gittuf verifies that there exists a path in the delegations
graph from the top level Targets role to the newly named policy, and that the
delegation path contains the keys presented for the new policy. If this check
succeeds, the new policy is created with the default `allow-rule`.

After a policy is initialized and stored in the gittuf policy namespace, new
protection rules can be added to the file. In each instance, the policy file is
re-signed, and therefore, authorized keys for that policy must be presented.

```bash
$ gittuf policy init
$ gittuf policy add-rule
$ gittuf policy remove-rule
```

Note: the commands listed here are examples and not exhaustive. Please refer to
gittuf's help documentation for more specific information about gittuf's usage.

### Recording updates in the RSL

The RSL records changes to the policy namespace automatically. To record changes
to other Git references, the developer must invoke the gittuf client and specify
the reference. gittuf then examines the reference and creates a new RSL entry.

Similarly, gittuf can also be invoked to create new RSL annotations. In this
case, the developer must specify the RSL entries the annotation applies to using
the target entries' Git identifiers.

```bash
$ gittuf rsl record
$ gittuf rsl annotate
```

Note: the commands listed here are examples and not exhaustive. Please refer to
gittuf's help documentation for more specific information about gittuf's usage.

### Syncing gittuf namespaces with the main repository

gittuf clients uses the `origin` Git remote to identify the main repository. As
the RSL must be linear with no branches, gittuf employs a variation of the
`Secure_Fetch` and `Secure_Push` workflows described in the RSL academic paper.

![gittuf-new-arch-alternate](https://github.com/gittuf/gittuf/assets/14241779/c01f438e-c4a0-4179-b570-4a39a215992b)

Note that gittuf can be used even if the main repository is not gittuf-enabled. The repository can host the gittuf namespaces which other gittuf clients can pull from for verification. In this example, a gittuf client with a changeset to commit to the dev branch (step 1), creates in its local repository a new commit object and the associated RSL entry (step 2). These changes are pushed next to a remote Git repository (step 3), from where other gittuf or legacy Git clients pull the changes (step 4).

#### RSLFetch: Receiving remote RSL changes

Before local RSL changes can be made or pushed, it is necessary to verify that
they are compatible with the remote RSL state. If the remote RSL has entries
that are unavailable locally, entries made locally will be rejected by the
remote. For example, let the local RSL tip be entry A and the new entry be entry
C. If the remote has entry B after A with B being the tip, attempting to push C
which also comes right after A will fail. Instead, the local RSL must first
fetch entry B and then create entry C. This is because entries in the RSL must
be made serially. As each entry includes the ID of the previous entry, a local
entry that does not incorporate the latest RSL entries on the remote is invalid.
The workflow is as follows:

1. Fetch remote RSL to the local remote tracker
   `refs/remotes/origin/gittuf/reference-state-log`.
1. If the last entry in the remote RSL is the same as the local RSL, terminate
   successfully.
1. Perform the verification workflow for the new entries in the remote RSL,
   incorporating remote changes to the local policy namespace. The verification
   workflow is performed for each Git reference in the new entries, relative to
   the local state of each reference. If verification fails, abort and warn
   user. Note that the verification workflow must fetch each Git reference to
   its corresponding remote tracker, `refs/remotes/origin/<ref>`. TODO: discuss
   if verification is skipped for entries that work with namespaces not present
   locally.
1. For each modified Git reference, update the local state. As all the refs have
   been successfully verified, each ref's remote state can be applied to the
   local repository, so `refs/heads/<ref>` matches `refs/remotes/origin/<ref>`.
1. Set local RSL to the remote RSL's tip.

#### RSLPush: Submitting local RSL changes

1. Execute `RSLFetch` repeatedly until there are no new RSL entries in the
   remote RSL. Every time there is a remote update, the user must be prompted to
   fetch and re-apply their changes to the RSL. This process could be automated
   but user intervention may be needed to resolve conflicts in the refs they
   modified. Changes to the gittuf policy must be fetched and applied locally.
1. Verify the validity of the RSL entries being submitted using locally
   available gittuf policies to ensure the user is authorized for the changes.
   If verification fails, abort and warn user.
1. For each new local RSL entry:
   1. Push the RSL entry to the remote. At this point, the remote is in an
      invalid state as changes to modified Git references have not been pushed.
      However, by submitting the RSL entry first, other gittuf clients that may
      be pushing to the repository must wait until this push is complete.
   1. If the entry is a normal entry, push the changes to the remote.
   1. TODO: discuss if RSL entries must be submitted one by one. If yes,
      `RSLFetch` probably needs to happen after each push. On the other hand, if
      all RSL entries are submitted first, other clients can recognize a push is
      in progress while other Git references are updated.

#### Invoking RSLFetch and RSLPush

While `RSLFetch` and `RSLPush` are invoked directly by the user to sync changes
with the remote, gittuf executes `RSLFetch` implicitly when a new RSL entry is
recorded. As RSL entries are typically recorded right before changes are
submitted to the remote, this ensures that new entries are created using the
latest remote RSL.

## Verification Workflow

There are several aspects to verification. First, the right policy state must be
identified by walking back RSL entries to find the last change to that
namespace. Next, authorized keys must be identified to verify that commit or RSL
entry signatures are valid.

### Identifying Authorized Signers for Protected Namespaces

When verifying a commit or RSL entry, the first step is identifying the set of
keys authorized to sign a commit or RSL entry in their respective namespaces.
With commits, the relevant namespaces pertain to the files they modify, tracked
by the repository. On the other hand, RSL entries pertain to Git refs. Assume
the relevant policy state entry is `P` and the namespace being checked is `N`.
Then:

1. Validate `P`'s Root metadata using the TUF workflow, ignore expiration date
   checks.
1. Begin traversing the delegations graph rooted at the top level Targets
   metadata. Set `current` to top level Targets and `parent` to Root metadata.
1. Create empty set `K` to record keys authorized to sign for `N`.
1. While `K` is empty:
   1. Load and verify signatures of `current` using keys provided in `parent`.
      Abort if signature verification fails.
   1. Identify delegation entry that matches `N`, `D`.
   1. If `D` is the `allow-rule`:
      1. Explicitly indicate any key is authorized to sign changes as `N` is not
         protected. Returning empty `K` alone is not sufficient.
   1. Else:
      1. If repository contains metadata with the role name in `D`:
         1. Set `parent` to `current`, `current` to delegatee role.
         1. Continue to next iteration.
      1. Else:
         1. Set `K` to keys authorized in the delegations entry.
1. Return `K`.

### Verifying Changes Made

In gittuf, verifying the validity of changes is _relative_. Verification of a
new state depends on comparing it against some prior, verified state. For some
ref `X` that is currently at verified entry `S` in the RSL and its latest
available state entry is `D`:

1. Fetch all changes made to `X` between the commit recorded in `S` and that
   recorded in `D`, including the latest commit into a temporary branch.
1. Walk back from `S` until a state entry `P` is found that updated the gittuf
   policy namespace. This identifies the policy that was active for changes made
   immediately after `S`.
1. Validate `P`'s metadata using the TUF workflow, ignore expiration date
   checks.
1. Walk back from `D` until `S` and create an ordered list of all state updates
   that targeted either `X` or the gittuf policy namespace. During this process,
   all state updates that affect `X` and the policy namespace must be recorded.
   Entries pertaining to other refs MAY be ignored. Additionally, all annotation
   entries must be recorded using a dictionary where the key is the ID of the
   entry referred to and the value the annotation itself. Each entry referred to
   in an annotation, therefore, must have a corresponding entry in the
   dictionary.
1. The verification workflow has an ordered list of states
   `[I1, I2, ..., In, D]` that are to be verified.
1. For each set of consecutive states starting with `(S, I1)` to `(In, D)`:
   1. Check if an annotation exists for the second state. If it does, verify if
      the annotation indicates the state is to be skipped. It true, proceed to
      the next set of consecutive states.
   1. If second state changes gittuf policy:
      1. Validate new policy metadata using the TUF workflow and `P`'s contents
         to established authorized signers for new policy. Ignore expiration
         date checks. If verification passes, update `P` to new policy state.
   1. Verify the second state entry was signed by an authorized key as defined
      in P.
   1. Enumerate all commits between that recorded in the first state and the
      second state with the signing key used for each commit. Verify each
      commit's signature using public key recorded in `P`.
   1. Identify the net or combined set of files modified between the commits in
      the first and second states as `F`.
   1. If all commits are signed by the same key, individual commits need not be
      validated. Instead, `F` can be used directly. For each path:
         1. Find the set of keys authorized to make changes to the path in `P`.
         1. Verify key used is in authorized set. If not, terminate verification
            workflow with an error.
   1. If not, iterate over each commit. For each commit:
      1. Identify the file paths modified by the commit. For each path:
         1. Find the set of keys authorized to make changes to the path in `P`.
         1. Verify key used is in authorized set. If not, check if path is
            present in `F`, as an unauthorized change may have been corrected
            subsequently. This merely acts as a hint as path may have been also
            changed subsequently by an authorized user, meaning it is in `F`. If
            path is not in `F`, continue with verification. Else, request user
            input, indicating potential policy violation.
   1. Set trusted state for `X` to second state of current iteration.

## Recovery

If every user were using gittuf and were performing each operation by
generating all of the correct metadata, following the specification, etc., then
the procedure for handling each situation is fairly straightforward. However,
an important property of gittuf is to ensure that a malicious or erroneous
party cannot make changes that impact the state of the repository in a negative
manner. To address this, this section discusses how to handle situations where
something has not gone according to protocol. The goal is to recover to a
"known good" situation which does match the metadata which a set of valid
gittuf clients would generate.

### Recovery Mechanisms

gittuf uses two basic mechanisms for recovery. We describe these core building
blocks for recovery before we discuss the exact scenarios when they are applied
and why they provide the desired security properties.

#### M1: Removing information to reset to known good state

This mechanism is utilized in scenarios where some change is rejected. For
example, one or more commits may have been pushed to a branch that do not meet
gittuf policy. The repository is updated such that these commits are neutralized
and all Git refs match their latest RSL entries. This can take two forms:

1. The rejected commit is removed and the state of the repo is set to the prior
commit which is known to be good. This is used when all rejected commits are
together at the end of the commit graph, making it easy to remove all of them.

2. The rejected commit is _reverted_ where a new commit is introduced that
reverses all the changes made in the reverted commit. This is needed when "good"
commits that must be retained are interspersed with "bad" commits that must be
rejected.

In both cases, new RSL entries and annotations may be used to record the
incident or to skip the invalid RSL entries corresponding to the rejected
changes. TODO: should these new RSL entries come from authorized users for the
affected namespace?

#### M2: Create RSL entry on behalf of another user

This mechanism is necessary for adoptions where a subset of developers do not
use gittuf. When they submit changes to the main copy of the repository, they do
not include RSL entries. Therefore, when a change is pushed to a branch by a
non-gittuf user A, a gittuf user B can submit an RSL entry on their behalf.
Additionally, the entry must identify the original user (TODO: cross-reference
with signed pushes?) and include some evidence about why B thinks the change
came from A (TODO: signed push can be evidence option?).

TODO: decide if B needs to be authorized for the affected branch to sign for A.

### Recovery Scenarios

These scenarios are some examples where recovery is necessary.

#### A change is made without an RSL entry

Bob does not use gittuf and pushes to a branch. Alice notices this as her gittuf
client detects a push to the branch without an accompanying RSL entry. She
validates that the change came from Bob and creates an RSL entry on his behalf,
identifying him and including information about how she verified it was him.
Alice applies M2.

#### An incorrect RSL entry is added

There are several ways in which an RSL entry can be considered "incorrect". If
an entry is malformed (structurally), Git may catch it if it's not a valid
commit. In such instances, the push from a buggy client is rejected altogether,
meaning other users are not exposed to the malformed commit.

Invalid entries that are not rejected by Git must be caught by gittuf. Some
examples of such invalid entries are:

* RSL entry is for a non-existing Git reference
* Commit recorded in RSL entry does not exist
* Commit recorded in RSL entry does not match the tip of the corresponding Git
  reference
* RSL annotation contains references to RSL entries that do not exist or are not
  RSL entries (i.e. the annotation points to other commits in the repository)

Note that as invalid RSL entries are only created by buggy or malicious gittuf
clients, these entries cannot be detected prior to them being pushed to the main
repository.

As correctly implemented gittuf clients verify the validity of RSL entries when
they pull from the main repository, the user is warned if invalid entries are
encountered. Then, the user can then use M1 to invalidate the incorrect entry.
Other clients with the invalid entry only need to fetch the latest RSL entries
to recover. Additionally, the client that created the invalid entries must
switch to a correct implementation of gittuf before further interactions with
the main repository.

If the main repository is also gittuf enabled, such incidents can be caught
before other users receive the incorrect RSL entries. The repository, though,
must not behave like a typical gittuf client. Instead, gittuf's repository
behavior is slightly different as RSL entries are submitted before the changes
they represent. The repository must wait to receive the full changes rather than
immediately rejecting the RSL entry. TODO: the repository-specific behavior
needs further discussion.

Consider this example as a representative of this scenario. Bob has a buggy
gittuf client and pushes an invalid entry to the main repository. Alice pulls
and receives a warning from her gittuf client. Alice reverses Bob's changes,
creating an RSL entry for the affected branch if needed, and includes an RSL
annotation skipping Bob's RSL entry.

#### A gittuf access control policy is violated

Bob creates an RSL entry for a branch he's not authorized for by gittuf policy.
He pushes a change to that branch. Alice notices this (TODO: decide if alice
needs to be authorized). Alice reverses Bob's change, creating a new RSL entry
reflecting that. Alice also creates an RSL annotation marking Bob's entry as
one to be skipped. Alice, therefore, uses M1.

#### Attacker modifies or deletes historical RSL entry

Overwriting or deleting an historical RSL entry is a complicated proposition.
Git's content addressable properties mean that a SHA-1 collision is necessary to
overwrite an existing RSL entry in the Git object store. Further, the attacker
also needs more than push access to the repository as Git will not accept an
object it already has in its store. Similarly, deleting an entry from the object
store preserves the RSL structure cosmetically but verification workflows that
require the entry will fail. This ensures that such an attack is detected, at
which point the owners of the repository can restore the RSL state from their
local copies.

Also note that while Git uses SHA-1 for its object store, cryptographic 
signatures are generated and verified using stronger hash algorithms. Therefore,
a successful SHA-1 collision for an RSL entry will not go undetected as all
entries are signed.

#### Dealing with fork* attacks

An attacker may attempt a forking attack where different developers receive
different RSL states. This is the case where the attacker wants to rewrite the
RSL's history by modifying an historical entry (which also requires a key
compromise so the attacker can re-sign the modified entry) or deleting it
altogether. To carry out this attack, the attacker must maintain and serve at
least two versions of the RSL. This is because at least one developer must have
the affected RSL entries--the author of the modified or deleted entries.
Maintaining and sending the expected RSL entry for each user is not trivial,
especially if multiple users have a version of the RSL without the attack. Also,
the attacker may be able to serve multiple versions of the RSL from a central
repository they control but any direct interactions between users that have the
original RSL and the attacked RSL will expose the attack.  These characteristics
indicate that while a fork* attack is not impossible, it is highly unlikely to
be carried out given its overhead and high chances of detection.

Finally, while gittuf primarily uses TUF's root of trust and delegations, it is
possible that TUF's timestamp role can be leveraged to further mitigate fork*
attacks. A future version of the gittuf specification may explore the use of the
timestamp role in this context.

#### An authorized key is compromised

When a key authorized by gittuf policies is compromised, it must be revoked and
rotated so that an attacker cannot use it to sign repository objects. gittuf
policies that grant permissions to the key must be updated to revoke the key,
possibly adding the actor's new key in the process. Further, if a security
analysis shows that the key was used to make malicious changes, those changes
must be reverted and the corresponding RSL entries signed with the compromised
key must be skipped. This ensures that gittuf clients do not consider attacker
created RSL entries as valid states for the corresponding Git references.
Clients that have an older RSL from before the attack can skip past the
malicious entries altogether.
