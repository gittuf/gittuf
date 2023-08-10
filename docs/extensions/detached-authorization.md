# Supporting Detached Authorizations for Actor Actions

Last Modified: August 10, 2023

Version: early draft, not ready for 0.1.0

Status: eventually-merge-into-specification

Certain actions in repositories need to be approved by multiple actors. In a
more general sense, TUF supports these notions using a "threshold" of
signatures, wherein a minimum number of signatures from an authorized set of
keys may be required for some role. Supporting this functionality in Git
repositories is complicated by the following factors:
* Git objects that support signatures can only embed a single signature.
* Git is typically (anecdotally?) used more asynchronously than a typical TUF
  deployment.

Note: it is possible that the final design of a detached "authorization" is an
application of a more general detached "signature". For now, the two are
designed in sync.

## Example Scenario

Consider a policy that spans the Git ref namespace and the files namespace. For
example, a repository may only allow Alice and Bob to merge to the main branch
and separately require Carol to authorize changes to the `foo` directory in the
main branch. Notice that Alice and Bob can merge to main but not make changes
to files in `foo`. On the other hand, Carol can make changes to files in `foo`
but cannot merge them to main herself. For a change to `foo` being merged to
the main branch to meet these policies, authorization is necessary from one of
Alice and Bob for the actual merge **and** Carol for the change to `foo`.
Authorization for merging to the main branch is verified using the signature on
the RSL entry, i.e., by confirming the signature was issued using either
Alice's or Bob's key. Authorization from Carol, however, is a separate matter.

### Strawman Solution

A strawman solution for this is as follows. The change to the main branch is
verified using the RSL entry. Separately, the change to `foo` is verified using
the corresponding commits. So, Carol makes the change to `foo` in some commit
and one of Alice and Bob merge that commit into main.

At face value, this seems to meet the desired requirement of a signature from
Carol for the change to `foo` and a signature from either Alice or Bob for the
merge to main. Unfortunately, while Carol can be confirmed as having made the
change, she does not authorize the application of the commit to the main
branch. Git commits are disassociated from branches. A commit can exist in
multiple branches or none at all (though Git's garbage collection will likely
change that). What this means is that Carol may make a change to `foo` that is
not appropriate for the main branch on a feature branch. Without her knowledge,
Alice or Bob can merge the change to main. This highlights that the strawman
solution does not enable the desired outcome for the specified policy.

Another flaw is that this design does not enable true multi-signing properties.
If the above policies were accompanied by another that said only David could
authorize changes to `foo` in the production branch, Carol's signature on the
commit changing `foo` is meaningless. David would have to resort to re-writing
and re-signing Carol's commit in this mechanism. On an ongoing basis, the two
branches would always differ even if the underlying tree objects are identical.
Git's fast forward features also become useless.

## Detached Authorization Structure

A detached authorization is issed by an actor for some action they must
authorize per policy. In the example described above, Carol would issue a
detached authorization for Alice or Bob to merge her change to `foo` into the
main branch. Detached authorizations must, therefore, unambiguously identify
the following features:
* the Git ref affected
* the target state of the Git ref, i.e. after applying the change
* the original state of the Git ref, i.e., prior to applying the change

Carol, therefore, states that the authorization is solely for the main branch,
and that the authorization is for changing the main branch to incorporate the
change to `foo`. Importantly, by identifying the original state of main, Carol
ensures that this authorization cannot be replayed to _rollback_ a future
change to `foo` after this one is merged. Specifically, let the original state
of main be A, Carol's commit be B which she authorizes as `(main, B, A)`.
Suppose there is a change C after B that also changes `foo` that she authorizes
as `(main, C, B)`. Alice or Bob must not be able to _reverse_ C, setting the
main branch to B without Carol's approval. If the detached authorization only
includes the target state of the ref, i.e. `(main, B)` and `(main, C)`, Carol's
approval to set main to B could be replayed during the rollback without her
knowledge.

TODO: nail down wireline format for authorization and how it is stored signed.

### Design Updates

Detached authorizations must identify:
* the Git ref affected
* the tip of the set of commits being applied (if commits A, B, C are to be
  applied to the ref, C is explicitly recorded, implicitly approving A and B)
* optionally, the last valid RSL entry for the Git ref affected

In effect, Carol says "I approve setting the main branch to commit C, where the
last valid RSL entry for main is X." Carol may omit the last valid entry for the
branch. This supports scenarios such as the first approval for a branch as well
as high traffic refs where other changes may be merged before Carol's in a
single go. When Carol does include the last RSL entry ID information, she
ensures her authorization cannot be replayed during a rollback attack.

In addition, Carol records commit C but the RSL entry for the merge may not
actually reflect C. This is the case when a merge commit is created. So, during
the verification workflow, if the RSL entry records a merge commit (identified
by having two parents in the affected branch), the second parent must match C.
If the RSL entry does not record a merge commit, then the commit must be C.
Significantly, this supports merge commits and fast forwards but NOT squash or
rebase workflows that re-create and apply A, B, C or (A + B + C) to the target
branch. Also, if Carol issues an authorization for C but A, B, and C must be
rebased to fix conflicts, Carol will need to issue a new authorization. Note
that if Carol includes the last valid RSL entry in her authorization, she must
issue new authorizations even if A, B, and C can be merged as-is.

## In Practice

Detached authorizations must be stored separately in
`refs/gittuf/authorizations`, tracked by the RSL. Each detached authorization
is stored as a blob object in a tree identified as `<target commit
ID>/<branch>:<original commit ID>`. This structure supports multiple
authorization per target commit for higher thresholds, different branches, and
_authorized_ rollbacks.

When a new authorization is added, the RSL records this change. Therefore, a
gittuf client verifying a change that requires authorization can access the
authorizations present in the repository via the RSL. This also means that
authorizations must be added to the repository _before_ the protected
action--Carol must upload an authorization for Alice or Bob to merge her change
to `foo` before they actually make that change. This has some implications,
discussed below.

## Open Questions

1. How are detached authorizations revoked?

The simplest option is for the author to _delete_ the authorization file. This
shows there are _inherent_ policies on the authorizations namespace. If Carol
issues an authorization, her key must also sign the RSL entry reflecting the
new authorization. The RSL entry recording her _removing one_ also requires her
signature. But what if an authorization must be removed after Carol is rotated
out? Does this namespace need a mix of explicit and implicit permissions?

Q. What is the _meaning_ of revoking a detached authorization? Should the
change be removed from the tree?

Thoughts: M1 is not an option. However, possibly M2 can be used to revert the
change that maps to an authorization. gittuf must check that a revocation
results in a less-than-threshold number of signatures before prompting M2.

Q. How does Carol know the target commit ID when merge commits are used? Should
gittuf handle the merge using fixed parameters?

One option is to have gittuf handle the merge and set the time etc. The author
is still an issue as it depends on which of Alice and Bob perform the merge.

Another option is to record the source and target tree objects. This needs
further consideration.

Yet another option is to incorporate _patches_. So Carol records the authorized
patch for a branch rather than the source and target commit or tree objects.

Q. Suppose Carol authorizes `(main, B, A)` but Bob has to rebase due to C
coming after A upstream. So, Carol then issues `(main, B', C)`. Should Carol
revoke her prior authorization to ensure a compromise of Alice or Bob's key
doesn't allow an attacker to replay her unused authorization?

It is probably best to revoke the authorization but in practice, what is
described here is a fork* attack. The gittuf specification discusses that
the complexity of such an attack is very high. It _might_ be okay for Carol to
ignore the authorization.
