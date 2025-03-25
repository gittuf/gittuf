# gittuf Design Document

Last Modified: March 25, 2025

## Introduction

This document describes gittuf, a security layer for Git repositories.
With gittuf, any developer who can pull from a Git repository can independently
verify that the repository's security policies were followed. gittuf's policy,
inspired by [The Update Framework (TUF)](https://theupdateframework.io/),
handles key management for all trusted developers in a repository, allows for
setting permissions for repository namespaces such as branches, tags, and files,
and provides protections against
[attacks targeting Git metadata](https://www.usenix.org/conference/usenixsecurity16/technical-sessions/presentation/torres-arias).
At the same time, gittuf is backwards compatible with existing source control
platforms ("forges") such as GitHub, GitLab, and Bitbucket. gittuf is currently
a sandbox project at the
[Open Source Security Foundation (OpenSSF)](https://openssf.org/) as part of the
[Supply Chain Integrity Working Group](https://github.com/ossf/wg-supply-chain-integrity).
The core concepts of gittuf described in this document have been
[peer reviewed](https://ssl.engineering.nyu.edu/papers/yelgundhalli_gittuf_ndss_2025.pdf).

This document is scoped to describing how gittuf's write access control policies
are applied to Git repositories. Other additions to gittuf's featureset are
described in standalone [gittuf Augmentation Proposals (GAPs)](/docs/gaps/).

## Definitions

This document uses several terms or phrases in specific ways. These are defined
here.

### Git References (Refs) and Objects

A Git reference is a "simple name" that typically points to a particular Git
commit. Generally, development in Git repositories are centered in one or more
refs, and they're updated as commits are added to the ref under development. By
default, Git defines two of refs: branches ("heads") and tags. Git allows for
the creation of other arbitrary refs that users can store other information as
long as they are formatted using Git's object types.

Git employs a content addressed object store, with support for four types of
objects. An essential Git object is the "commit", which is a self-contained
representation of the whole repository. Each commit points to a "tree" object
that represents the state of the files in the root of the repository at that
commit. A tree object contains one or more entries that are either other tree
objects (representing subdirectories) or "blob" objects (representing files).
The final type of Git object is the "tag" object, used as a static pointer to
another Git object. While a tag object can point to any other Git object, it is
frequently used to point to a commit.

```
Repository
|
|-- refs
|   |
|   |-- heads
|   |   |-- main (refers to commit C)
|   |   |-- feature-x (refers to commit E)
|   |
|   |-- tags
|   |   |-- v1.0 (refers to tag v1.0)
|   |
|   |-- arbitrary
|       |-- custom-ref (formatted as Git object type)
|
|-- objects
    |-- A [Initial commit]
    |-- B [Version 1.0 release]
    |-- C [More changes on main]
    |-- D [Initial commit on feature-x]
    |-- E [More changes on feature-x]
    |-- v1.0 [Tag object referring to commit B]
```


### Actors and Authentication

In a Git repository, an "actor" is any party, human or bot, who makes changes to
the repository. These changes can involve any part of the repository, such as
modifying files, branches or tags. In gittuf, each actor is identified by a
unique signing key that they use to cryptographically sign their contributions.
gittuf uses cryptographic signatures to authenticate actors as these signatures
can be verified by anyone who has the corresponding public key, fundamental to
gittuf's mission to enable independent verification of repository actions. Note
that gittuf does not rely on Git commit metadata (e.g., author email, committer
email) to identify the actor who created it, as that may be trivially spoofed.

In practice, a gittuf policy allows an actor to make certain changes by granting
trust to the actor's signing key to make those changes. To maintain security,
all actions made in the repository, such as adding or modifying files, are
checked for authenticity. This is done by verifying the digital signature
attached to the action, which must match the trusted public key associated with
the actor who is supposed to have made the change.

### State

The term "state" refers to the latest values or conditions of the tracked
references (like branches and tags) in a Git repository.  These are determined
by the most recent entries in the
[reference state log](#reference-state-log-rsl). Note that when verifying
changes in the repository, a workflow may only verify specific references rather
than all state updates in the reference state log.

## Threat Model

The following threat model is taken from the
[peer reviewed publication](https://ssl.engineering.nyu.edu/papers/yelgundhalli_gittuf_ndss_2025.pdf)
describing gittuf.

We consider the standard scenario where a forge is used to manage a Git
repository on a centralized synchronization point. This forge can be a publicly
hosted solution (e.g., the github.com service), or self-hosted on premises by an
enterprise. Either option exposes the forge instance to both external attackers
and insider threats. External attackers may circumvent security measures and
compromise the version control system, manifesting themselves as advanced
persistent threats (APT) and making unnoticed changes to the system. Similarly,
insider threats may be posed by rogue employees with escalated privileges who
abuse their authority to make unnoticed changes.

To protect the integrity of the repository’s contents, the maintainers of the
repository define security controls such as which contributors can write to
different parts of the repository. gittuf is meant to protect against scenarios
where any party, individual developers, bots that make changes, or the forge
itself, may be compromised and act in an arbitrarily malicious way as seen in
prior incidents. This includes scenarios such as:

* T1: Modifying configured repository security policies, such as to weaken them
* T2: Tampering with the contents of the repository’s activity log, such as by
      reordering, dropping, or otherwise manipulating log entries
* T3: Subverting the enforcement of security policies, such as by accepting
      invalid changes instead of rejecting them

Note that we consider out of scope a freeze attack, where the forge serves stale
data, as development workflows involve a substantial amount of out-of-band
communication which prevents such attacks from going unnoticed. We similarly
consider weaknesses in cryptographic algorithms as out of scope.

## gittuf Design

gittuf records additional metadata describing the repository's policy and
activity in the repository itself. Effectively, gittuf treats security policies,
activity information, and policy decisions as a content tracking problem. To
avoid collisions with regular repository contents, gittuf stores its metadata in
custom references under `refs/gittuf/`.

### gittuf Policy

Note: This section assumes some prior knowledge of the
[TUF specification](https://theupdateframework.github.io/specification/).

The repository's policy metadata handles the distribution of the repository's
trusted keys (representing actors) as well as write access control rules. There
are two types of metadata used by gittuf, which are stored in a custom reference
`refs/gittuf/policy`.

#### Root of Trust

gittuf's policy metadata includes root of trust metadata, which
establishes why the policy must be trusted. The root of trust metadata (similar
to TUF's root metadata) declares the keys belonging to the repository owners as
well as a numerical threshold that indicates the minimum number of signatures
for the metadata to be considered valid. The root of trust metadata is signed by
a threshold of root keys, and the initial set of root keys for a repository must
be distributed using out-of-band mechanisms or rely on trust-on-first-use
(TOFU). Subsequent changes to the set of root keys are handled in-band, with a
new version of the root of trust metadata created. This new version must be
signed by a threshold of root keys trusted in the previous version.

#### Rule Files

The rules protecting the repository's namespaces are declared in one or more
rule files. A rule file is similar to TUF's targets metadata. It declares the
public keys for the trusted actors, as well as namespaced "delegations" which
specify protected namespaces within the repository and which actors are trusted
to write to them.

A threshold of trusted actors for any delegation (or rule) can extend this trust
to other actors by signing a new rule file with the same name as the delegation.
In this rule file, they can add the actors who must be trusted for the same (or
a subset) of namespaces.

All repositories must contain a primary rule file (typically called
"targets.json" to match TUF's behavior). This rule file may contain no rules,
signifying that no repository namespaces are protected. The primary rule file
derives its trust directly from the root of trust metadata; it must be signed by
a threshold of actors trusted to manage the repository's primary rule file. All
other rule files derive their trust directly or indirectly from the primary rule
file through delegations.

![Policy delegations](/docs/media/policy-delegations.png)
_In this example, the repository administrator grants write permissions to Carol 
for the main branch, to Alice for the alice-dev branch, and to Bob for the
/tests folder (under any of the existing branches)._

A significant difference between typical TUF metadata and those used by gittuf
is in the expectations of the policies. Typical TUF deployments are explicit
about the artifacts they are distributing. Any artifact not listed in TUF
metadata is rejected. In gittuf, policies are written only to express
_restrictions_. As such, when verifying changes to unprotected namespaces,
gittuf must allow any key to sign for these changes. This means that after all
explicit policies (expressed as delegations) are processed, and none apply to
the namespace being verified, an implicit `allow-rule` is applied, allowing
verification to succeed.

#### Example gittuf Policy

The following example is taken from the
[peer reviewed publication](https://ssl.engineering.nyu.edu/papers/yelgundhalli_gittuf_ndss_2025.pdf)
of gittuf's design. It shows a gittuf policy state with its root of trust and
three distinct rule files connected using delegations. The root of trust
declares the trusted signers for the next version of the root of trust as well
as the primary rule file. Signatures are omitted.

```
rootOfTrust:
keys: {R1, R2, R3, P1, P2, P3}
signers:
    rootOfTrust: (2, {R1, R2, R3})
    primary: (2, {P1, P2, P3})

ruleFile: primary
keys: {Alice, Bob, Carol, Helen, Ilda}
rules:
    protect-main-prod: {git:refs/heads/main,
                        git:refs/heads/prod}
        -> (2, {Alice, Bob, Carol})
    protect-ios-app: {file:ios/*}
        -> (1, {Alice})
    protect-android-app: {file:android/*}
        -> (1, {Bob})
    protect-core-libraries: {file:src/*}
        -> (2, {Carol, Helen, Ilda})

ruleFile: protect-ios-app
keys: {Dana, George}
rules:
    authorize-ios-team: {file:ios/*}
        -> (1, {Dana, George})

ruleFile: protect-android-app
keys: {Eric, Frank}
rules:
    authorize-android-team: {file:android/*}
        -> (1, {Eric, Frank})
```

### Tracking Repository Activity

gittuf leverages a "Reference State Log (RSL)" to track changes to the
repository's references. In addition, gittuf uses the
[in-toto Attestation Framework](https://github.com/in-toto/attestation) to
record other repository activity such as code review approvals.

#### Reference State Log (RSL)

Note: This document presents a summary of the RSL. For a full understanding of
the attacks mitigated by the RSL, please refer to the
[academic](https://www.usenix.org/system/files/conference/usenixsecurity16/sec16_paper_torres-arias.pdf)
[papers](https://ssl.engineering.nyu.edu/papers/yelgundhalli_gittuf_ndss_2025.pdf)
underpinning gittuf's design.

The Reference State Log contains a series of entries that each describe some
change to a Git ref. Such entries are known as RSL reference entries. Each entry
contains the ref being updated, the new location it points to, and a hash of the
parent RSL entry. The entry is signed by the actor making the change to the ref.

Additionally, the RSL supports annotation entries that refer to prior reference
entries. An annotation entry can be used to attach additional user-readable
messages to prior RSL entries or to mark those entries as revoked.

Given that each entry points to its parent entry using its hash, an RSL is a
hash chain. gittuf's implementation of the RSL uses Git's underlying Merkle
graph. Generally, gittuf is designed to ensure the RSL is linear but a
privileged attacker may be able to cause the RSL to branch, resulting in a fork*
attack where different actors are presented different versions of the RSL. The
feasibility and implications of such an attack are discussed later in this
document.

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
gittuf design.

##### RSL Reference Entries

These entries are the standard variety described above. They contain the name of
the reference they apply to and a target ID. As such, they have the following
structure.

```
RSL Reference Entry

ref: <ref name>
targetID: <target ID>
number: <number>
```

The `targetID` is typically the ID of a commit for references that are branches.
However, for entries that record the state of a Git tag, `targetID` is the ID of
the annotated tag object.

##### RSL Annotation Entries

Apart from regular entries, the RSL can include annotations that apply to prior
RSL entries. Annotations can be used to add more information as a message about
a prior entry, or to _explicitly_ mark one or more entries as ones to be
skipped. This semantic is necessary when accidental or possibly malicious RSL
entries are recorded. Since the RSL history cannot be overwritten, an annotation
entry must be used to communicate to gittuf clients to skip the corresponding
entries. Annotations have the following schema.

```
RSL Annotation Entry

entryID: <RSL entry ID 1>
entryID: <RSL entry ID 2>
...
skip: <true/false>
number: <number>
-----BEGIN MESSAGE-----
<message>
------END MESSAGE------
```

##### Example Entries

Here's a sample RSL, with the output taken from `gittuf rsl log`:

```
entry a5ea2c6ee7e8b577f6be6fcee5b06e6cac7166fa (skipped)

  Ref:    refs/heads/main
  Target: 6cb8e5c546eab3d0e1d245014de8003febb8e9b3
  Number: 5

    Annotation ID: cccfb6f27b2a71c33e9a55bc82f084e2445aa398
    Skip:          yes
    Number:        6
    Message:
      Skipping RSL entry

entry 40c82851f78c7018f4c360030a83923b0925c28d

  Ref:    refs/gittuf/policy
  Target: b7cf91ec9b5b6b17334ab1378dc85375236524f5
  Number: 4

entry 94c153bff6d684a956ed27f0abd70624e875657c

  Ref:    refs/gittuf/policy-staging
  Target: b7cf91ec9b5b6b17334ab1378dc85375236524f5
  Number: 3

entry fed977a5ca07e566af3a37808284dc7c5a67d6dc

  Ref:    refs/gittuf/policy-staging
  Target: dcbb536bd86a69e555692aec7b65c20de8257ee2
  Number: 2

entry e026a62f1c63c6db58bb357f9a85cafe05c64fb6

  Ref:    refs/gittuf/policy-staging
  Target: 603fc733218a0a1e54ccde47d1d9864f67e0bb75
  Number: 1
```

Specifically, the latest reference entry
`a5ea2c6ee7e8b577f6be6fcee5b06e6cac7166fa` has been skipped by an annotation
entry `cccfb6f27b2a71c33e9a55bc82f084e2445aa398`.

The commit object for the reference entry is as follows:

```bash
~/tmp/repo $ git cat-file -p a5ea2c6ee7e8b577f6be6fcee5b06e6cac7166fa
tree 4b825dc642cb6eb9a060e54bf8d69288fbee4904
parent 40c82851f78c7018f4c360030a83923b0925c28d
author Aditya Sirish A Yelgundhalli <ayelgundhall@bloomberg.net> 1729514863 -0400
committer Aditya Sirish A Yelgundhalli <ayelgundhall@bloomberg.net> 1729514863 -0400
gpgsig -----BEGIN SSH SIGNATURE-----
 U1NIU0lHAAAAAQAAADMAAAALc3NoLWVkMjU1MTkAAAAg8g2CmHSb7guzi6MUNgwHUQnxPN
 X1x8urScZyJrUB6MMAAAADZ2l0AAAAAAAAAAZzaGE1MTIAAABTAAAAC3NzaC1lZDI1NTE5
 AAAAQGQMSviwqF+cE/wgEo0U73vu86YHi4f5crzzFIctjyMGOOy2isYfHgGvSzs5bv6V2Q
 EtMumBSVbCxvnRqJpiFAs=
 -----END SSH SIGNATURE-----

RSL Reference Entry

ref: refs/heads/main
targetID: 6cb8e5c546eab3d0e1d245014de8003febb8e9b3
number: 5
```

Similarly, the commit object for the annotation entry is as follows:

```bash
~/tmp/repo $ git cat-file -p cccfb6f27b2a71c33e9a55bc82f084e2445aa398
tree 4b825dc642cb6eb9a060e54bf8d69288fbee4904
parent a5ea2c6ee7e8b577f6be6fcee5b06e6cac7166fa
author Aditya Sirish A Yelgundhalli <ayelgundhall@bloomberg.net> 1729514924 -0400
committer Aditya Sirish A Yelgundhalli <ayelgundhall@bloomberg.net> 1729514924 -0400
gpgsig -----BEGIN SSH SIGNATURE-----
 U1NIU0lHAAAAAQAAADMAAAALc3NoLWVkMjU1MTkAAAAg8g2CmHSb7guzi6MUNgwHUQnxPN
 X1x8urScZyJrUB6MMAAAADZ2l0AAAAAAAAAAZzaGE1MTIAAABTAAAAC3NzaC1lZDI1NTE5
 AAAAQNf32yJvhGfLIIeeStHgkSB7iuRGJl6LhbRTpX/q49lUu4TrEiCeGa3H8LMJ/5D1EE
 in3QAhlzdowYnmCKglTAw=
 -----END SSH SIGNATURE-----

RSL Annotation Entry

entryID: a5ea2c6ee7e8b577f6be6fcee5b06e6cac7166fa
skip: true
number: 6
-----BEGIN MESSAGE-----
U2tpcHBpbmcgUlNMIGVudHJ5
-----END MESSAGE-----
```

#### Attestations for Authorization Records

gittuf makes use of the signing capability provided by Git for commits and tags
significantly. However, it is sometimes necessary to attach more than a single
signature to a Git object or repository action. For example, a policy may
require more than one developer to sign off and approve a change such as merging
something to the `main` branch. To support these workflows (while also remaining
compatible with standard Git clients), gittuf uses the concept of "detached
authorizations", implemented using signed [in-toto
attestations](https://github.com/in-toto/attestation). Attestations are tracked
in the custom Git reference `refs/gittuf/attestations`. The gittuf design
currently supports the "reference authorization" type to represent code review
approvals. Other types may be added to this document or via [GAPs](/docs/gaps/)
in future.

A reference authorization is an attestation that accompanies an RSL reference
entry, allowing additional developers to issue signatures authorizing the change
to the Git reference in question. Its structure is similar to that of a
reference entry:

```
TargetRef    string
FromTargetID string
ToTargetID   string
```

The `TargetRef` is the Git reference the authorization is for, while
`FromTargetID` and `ToTargetID` record the change in the state of the reference
authorized by the attestation (as Git hashes). The information pertaining to the
prior state of the Git reference is explicitly recorded in the attestation
unlike a standard RSL reference entry. This is because, for a reference entry,
this information can be implicitly identified using the RSL by examining the
previous entry for the reference in question. If the authorization is for a
brand new reference (say a new branch or any tag), `FromTargetID` must be set to
zero. For a change to a branch, `ToTargetID` pre-computes the Git merge tree
resulting from the change being approved. Thus, when verifying the change to the
branch, it must be followed by an RSL reference entry that points to a commit
which has the same Git tree ID. For a tag, `ToTargetID` records the Git object
the tag object is expected to point to.

Reference authorizations are stored in a directory called
`reference-authorizations` in the attestations namespace. Each authorization
must have the in-toto predicate type:
`https://gittuf.dev/reference-authorization/v<VERSION>`.

## gittuf Workflows

gittuf introduces some new workflows that are gittuf-specific, such as the
creation of policies and their verification. In addition, gittuf interposes in
some Git workflows so as to capture repository activity information.

### Policy Initialization and Changes

When the policy is initialized or updated (this can be a change to the root of
trust metadata or one or more rule files), a new policy state is created that
contains the full set of gittuf policy metadata. This is recorded as a commit in
the custom ref used to track the policy metadata (typically
`refs/gittuf/policy`). In turn, the commit to the custom ref is recorded in the
RSL, indicating the policy state to use for subsequent changes in the
repository.

### Syncing gittuf References

As the RSL must be linear with no branches, gittuf employs a variation of the
`Secure_Fetch` and `Secure_Push` workflows described in the
[RSL academic paper](https://www.usenix.org/system/files/conference/usenixsecurity16/sec16_paper_torres-arias.pdf).

![Using gittuf with legacy servers](/docs/media/gittuf-with-legacy-servers.png)
_Note that gittuf can be used even if the synchronization point is not
gittuf-enabled. The repository can host the gittuf namespaces which other gittuf
clients can pull from for verification. In this example, a gittuf client with a
changeset to commit to the dev branch (step 1), creates in its local repository
a new commit object and the associated RSL entry (step 2). These changes are
pushed next to a remote Git repository (step 3), from where other gittuf or
legacy Git clients pull the changes (step 4)._

#### `RSLFetch`: Receiving Remote RSL Changes

Before local RSL changes can be made or pushed, it is necessary to verify that
they are compatible with the remote RSL state. If the remote RSL has entries
that are unavailable locally, entries made locally will be rejected by the
remote. For example, let the local RSL tip be entry A and the new entry be entry
C. If the remote has entry B after A with B being the tip, attempting to push C
which also comes right after A will fail. Instead, the local RSL must first
fetch entry B and then create entry C. This is because entries in the RSL must
be made serially. As each entry includes the ID of the previous entry, a local
entry that does not incorporate the latest RSL entry on the remote is invalid.
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
   its corresponding remote tracker, `refs/remotes/origin/<ref>`.
1. For each modified Git reference, update the local state. As all the refs have
   been successfully verified, each ref's remote state can be applied to the
   local repository, so `refs/heads/<ref>` matches `refs/remotes/origin/<ref>`.
1. Set local RSL to the remote RSL's tip.

NOTE: Some aspects of this workflow are under discussion and are subject to
change. The gittuf implementation does not implement precisely this workflow.
Specifically, the implementation does not verify new entries in the remote
automatically. Additionally, the RSL may contain entries for references a client
does not have, making verification of those entries unfeasible. See
https://github.com/gittuf/gittuf/issues/708.

#### `RSLPush`: Submitting Local RSL Changes

1. Execute `RSLFetch` repeatedly until there are no new RSL entries in the
   remote RSL. Every time there is a remote update, the user must be prompted to
   fetch and re-apply their changes to the RSL. This process could be automated
   but user intervention may be needed to resolve conflicts in the refs they
   modified. Changes to the gittuf policy must be fetched and applied locally.
1. Verify the validity of the RSL entries being submitted using locally
   available gittuf policies to ensure the user is authorized for the changes.
   If verification fails, abort and warn user.
1. Perform an atomic Git push to the remote of the RSL as well as the modified
   Git references. If the push fails, it is likely because another actor pushed
   their changes first. Restart the `RSLPush` workflow.

NOTE: Some aspects of this workflow are under discussion and are subject to
change. The gittuf implementation does not implement precisely this workflow.
This workflow is closely related to other push operations performed in the
repository, and therefore, this section may be incorporated with other
workflows. See https://github.com/gittuf/gittuf/issues/708.

### Regular Pushes

When an actor pushes a change to a remote repository, this update to the
corresponding ref (or refs) must be recorded in the RSL. For each ref being
pushed, the gittuf client creates a new RSL entry. Then, `RSLPush` is used to
submit these changes to the remote repository.

### Force Pushes

Due to the linear nature of the RSL, it is not possible to remove a reference
entry. A force push makes one or more prior reference entries for the pushed ref
invalid as the targets recorded in those entries may not be reachable any
longer. Thus, these entries must be marked as "skipped" in the RSL using an
annotation entry. After an annotation for these reference entries is created, a
reference entry is created recording the current state of the ref. Then,
`RSLPush` is used to submit these changes to the remote repository.

### Verification Workflow

There are several aspects to verification. First, the right policy state must be
identified by walking back RSL entries to find the last change to that
namespace. Next, authorized keys must be identified to verify that commit or RSL
entry signatures are valid.

#### Identifying Authorized Signers for Protected Namespaces

When verifying a commit or RSL entry, the first step is identifying the set of
keys authorized to sign a commit or RSL entry in their respective namespaces.
This is achieved by performing pre-ordered depth first search over the
delegations graph in a gittuf policy state. Assume the relevant policy state
entry is `P` and the namespace being checked is `N`. Then:

1. Validate `P`'s root metadata using the TUF workflow starting from the initial
   root of trust metadata, ignore expiration date checks (see
   https://github.com/gittuf/gittuf/issues/280).
1. Create empty set `K` to record authorized verifiers for `N`.
1. Create empty set `queue` to track the rules (or delegations) that must be
   checked.
1. Begin traversing the delegations graph rooted at the primary rule file
   metadata.
1. Verify the signatures of the primary rule file using the trusted keys in the
   root of trust. If a threshold of signatures cannot be verified, abort.
1. Populate `queue` with the rules in the primary rule file.
1. While `queue` is not empty:
   1. Set `rule` to the first item in `queue`, removing it from `queue`.
   1. If `rule` is the `allow-rule`:
      1. Proceed to the next iteration.
   1. If the patterns of `rule` match `N` (i.e., the rule applies to the
      namespace being verified):
      1. Create a verifier with the trusted keys in `rule` and the specified
         threshold.
      1. Add this verifier to `K`.
      1. If `P` contains a rule file with the same name as `rule` (i.e., a
         delegated rule file exists):
         1. Verify that the delegated rule file is signed by a threshold of
            valid signatures using the keys declared in delegating rule file.
            Abort if verification fails.
         1. Add the rules in `current` to the front of `queue` (ensuring the
            delegated rules are prioritized to match pre-order depth first
            search behavior).
1. Return `K`.

#### Verifying Changes Made

In gittuf, verifying the validity of changes is _relative_. Verification of a
new state depends on comparing it against some prior, verified state. For some
ref `X` that is currently at verified entry `S` in the RSL and its latest
available state entry is `D`:

1. Fetch all changes made to `X` between the commit recorded in `S` and that
   recorded in `D`, including the latest commit into a temporary branch.
1. Walk back from `S` until an RSL entry `P` is found that updated the gittuf
   policy namespace. This identifies the policy that was active for changes made
   immediately after `S`. If a policy entry is not found, abort.
1. Walk back from `S` until an RSL entry `A` is found that updated the gittuf
   attestations ref. This identifies the set of attestations applicable for the
   changes made immediately after `S`.
1. Validate `P`'s metadata using the TUF workflow, ignore expiration date
   checks (see https://github.com/gittuf/gittuf/issues/280).
1. Walk back from `D` until `S` and create an ordered list of all RSL updates
   that targeted either `X` or gittuf namespaces. Entries pertaining to other
   refs MAY be ignored. Annotation entries MUST be recorded.
1. The verification workflow has an ordered list of states
   `[I1, I2, ..., In, D]` that are to be verified.
1. Set trusted set for `X` to `S`.
1. For each set of consecutive states starting with `(S, I1)` to `(In, D)`:
   1. Check if an annotation exists for the second state. If it does, verify if
      the annotation indicates the state is to be skipped. It true, proceed to
      the next set of consecutive states.
   1. If second state changes gittuf policy:
      1. Validate new policy metadata using the TUF workflow and `P`'s contents
         to established authorized signers for new policy. Ignore expiration
         date checks (see https://github.com/gittuf/gittuf/issues/280). If
         verification passes, update `P` to new policy state.
   1. If second state is for attestations:
      1. Set `A` to the new attestations state.
   1. Verify the second state entry was signed by an authorized key as defined
      in `P` for the ref `X`. If the gittuf policy requires more than one
      signature, search for a reference authorization attestation for the same
      change. Verify the signatures on the attestation are issued by authorized
      keys to meet the threshold, ignoring any signatures from the same key as
      the one used to sign the entry.
   1. If `P` contains rules protecting files in the repository:
      1. Enumerate all commits between that recorded in trusted state and the
         second state with the signing key used for each commit.
      1. Identify the net or combined set of files modified between the commits
         in the first and second states as `F`.
      1. If all commits are signed by the same key, individual commits need not
         be validated. Instead, `F` can be used directly. For each path:
            1. Find the set of keys authorized to make changes to the path in
               `P`.
            1. Verify key used is in authorized set. If not, terminate
               verification workflow with an error.
      1. If not, iterate over each commit. For each commit:
         1. Identify the file paths modified by the commit. For each path:
            1. Find the set of keys authorized to make changes to the path in
               `P`.
            1. Verify key used is in authorized set. If not, check if path is
               present in `F`, as an unauthorized change may have been corrected
            subsequently. This merely acts as a hint as path may have been also
            changed subsequently by an authorized user, meaning it is in `F`. If
            path is not in `F`, continue with verification. Else, request user
            input, indicating potential policy violation.
      1. Set trusted state for `X` to second state of current iteration.
1. Return indicating successful verification.

NOTE: Some aspects of this workflow are under discussion and are subject to
change. The gittuf implementation does not implement precisely this workflow,
instead also including aspects of the recovery workflow to see if a change that
fails verification has already been recovered from. See
https://github.com/gittuf/gittuf/issues/708.

### Recovery

If every user were using gittuf and were performing each operation by
generating all of the correct metadata, following the specification, etc., then
the procedure for handling each situation is fairly straightforward. However,
an important property of gittuf is to ensure that a malicious or erroneous
party cannot make changes that impact the state of the repository in a negative
manner. To address this, this section discusses how to handle situations where
something has not gone according to protocol. The goal is to recover to a
"known good" situation which does match the metadata which a set of valid
gittuf clients would generate.

#### Recovery Mechanisms

When gittuf verification fails, the following recovery workflow must be
employed. This mechanism is utilized in scenarios where some change is rejected.
For example, one or more commits may have been pushed to a branch that do not
meet gittuf policy. The repository is updated such that these commits are
neutralized and all Git refs match their latest RSL entries. This can take two
forms:

1. The rejected commit is removed and the state of the repo is set to the prior
commit which is known to be good. This is used when all rejected commits are
together at the end of the commit graph, making it easy to remove all of them.

2. The rejected commit is _reverted_ where a new commit is introduced that
reverses all the changes made in the reverted commit. This is needed when "good"
commits that must be retained are interspersed with "bad" commits that must be
rejected.

In both cases, new RSL entries and annotations must be used to record the
incident and skip the invalid RSL entries corresponding to the rejected changes.

gittuf, by default, prefers the second option, with an explicit revert commit
that is tree-same as the last good commit. This ensures that a client can always
fast-forward to a fix rather than rewind. By resetting the affected branch to a
prior good commit, Git clients that have already pulled in the invalid commit
will not reset as well. Instead, they will assume they are ahead of the remote
in question and will continue to use the bad commit as the latest commit.

When the gittuf verification workflow encounters an RSL entry for some Git
reference that does not meet policy, it looks to see if a subsequent entry for
the same reference contains a fix that aligns with the last known good state.
Any intermediate entries between the original invalid entry and the fix for the
reference in question are also considered to be invalid. Therefore, in addition
to the fix RSL entry, gittuf also expects skip annotations for the original
invalid entry and intermediate entries for the reference.

#### Recovery Scenarios

These scenarios are some examples where recovery is necessary. This is not meant
to be an exhaustive set of gittuf's recovery scenarios.

##### An Incorrect RSL Entry is Added

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
clients, these entries cannot be detected prior to them being pushed to the
synchronization point.

As correctly implemented gittuf clients verify the validity of RSL entries when
they pull from the synchronization point, the user is warned if invalid entries
are encountered. Then, the user can then use the recovery workflow to invalidate
the incorrect entry. Other clients with the invalid entry only need to fetch the
latest RSL entries to recover. Additionally, the client that created the invalid
entries must switch to a correct implementation of gittuf before further
interactions with the main repository, but this is left to out-of-band
synchronization between the actors who notice the issue and the actor using a
buggy client.

##### A gittuf Access Control Policy is Violated

An actor, Bob, creates an RSL entry for a branch he's not authorized for by
gittuf policy. He pushes a change to that branch. Another actor, Alice, notices
this when her gittuf client indicates a failure in the verification workflow.
Alice creates an RSL annotation marking Bob's entry as one to be skipped. Alice
also reverses Bob's change, creating a new RSL entry reflecting that.

##### Attacker Modifies or Deletes Historical RSL Entry

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

##### Forge Attempts Fork* Attacks

An attacker who controls the forge may attempt a fork* attack where different
developers receive different RSL states. For example, the attacker may drop a
push from an actor, Alice, from the RSL. Other developers such as Bob and Carol
would continue adding their RSL entries, unaware of the dropped entry. However,
Alice will observe the divergence in the RSL as she cannot receive Bob's and
Carol's changes.

The attacker cannot simply reapply Bob's and Carol's changes over Alice's RSL
entry without also controlling Bob's and Carol's keys. The attacker may attempt
a freeze attack targeted against Alice, where she's always told her entry is the
latest in the RSL. However, any out-of-band communication between Alice and
either Bob or Carol (common during development workflows) will highlight the
attack.

##### An Authorized Key is Compromised

When a key authorized by gittuf policy is compromised, it must be revoked and
rotated so that an attacker cannot use it to sign repository objects. gittuf
policies that grant permissions to the key must be updated to revoke the key,
possibly adding the actor's new key in the process. Further, if a security
analysis shows that the key was used to make malicious changes, those changes
must be reverted and the corresponding RSL entries signed with the compromised
key must be skipped. This ensures that gittuf clients do not consider attacker
created RSL entries as valid states for the corresponding Git references.
Clients that have an older RSL from before the attack can skip past the
malicious entries altogether.

## Example of Using gittuf

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
