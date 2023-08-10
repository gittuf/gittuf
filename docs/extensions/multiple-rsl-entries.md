# Using Multiple RSL Entries for Threshold Authorizations

Last Modified: August 10, 2023

Version: early draft, not ready for 0.1.0

Status: eventually-merge-into-specification

Each party that needs to authorize a merge signs a distinct RSL entry.  For
example, if only Alice is authorized to merge changes to main and all changes to
file foo require sign off from Bob, after the change to foo is made, both Alice
and Bob issue identical (in content) RSL entries for main.

This design means there's a period of time where the RSL contains only one of
the two necessary entries. In the standard verification workflow, this would
trigger an error. To avoid this, the RSL entry schema is updated to include a
new field that indicates whether an entry must be processed in isolation or with
other entries. Suppose Alice's entry is submitted first. As it indicates it must
be processed with more entries, gittuf clients recognize that a threshold of
signatures are not present but do not error out. However, note that the client
also does not apply the entry's commit to the branch. After Bob's entry is also
added and the client fetches it, it re-runs verification and with a sufficient
number of signatures, the change is applied to main.

By not raising an error with only Alice's entry, clients can process other
subsequent entries between Alice's and Bob's entries. This ensures that clients
are not frozen by a delay in Bob's authorization.

## In Action

1. Alice needs to sign off on `main`. Bob needs to sign off on changes to `foo`.
Alice signs an entry for `main` with the commit ID. Bob signs an entry for
`main` with the same commit ID after Alice.

```
-----------------      ---------------
| Alice's entry | <--- | Bob's entry |
-----------------      ---------------
```

gittuf clients performing verification obtain both entries and verify them
together rather than in isolation.

```
verifySignatures([Alice's entry, Bob's entry], threshold=2)
```

2. Alice needs to sign off on `main`. Bob needs to sign off on changes to `foo`.
Alice signs an entry for `main` with the commit ID. Before Bob can sign his
entry, Carol submits an entry to the RSL after Alice's for a different branch.

```
-----------------      -----------------      ---------------
| Alice's entry | <--- | Carol's entry | <--- | Bob's entry |
-----------------      -----------------      ---------------
```

gittuf clients receive all three entries. However, the client takes a pass at
the list of entries and rearranges them so that the entries that need to be
verified together are collated.

```
verifySignatures([Alice's entry, Bob's entry], threshold=2)
verifySignatures([Carol's entry], threshold=1)
```

The order of verification uses the _earliest_ instance of the set of RSL
entries. Specifically, Alice's and Bob's entries are combined and verified
before Carol's entry is verified.

3. Alice needs to sign off on `main`. Bob needs to sign off on changes to `foo`.
Alice signs an entry for `main` with the commit ID. However, Bob is delayed from
creating his entry. Carol submits an entry to the RSL after Alice's for a
different branch.

```
-----------------      -----------------
| Alice's entry | <--- | Carol's entry |
-----------------      -----------------
```

gittuf clients know by policy that Alice's change needs to be accompanied by a
signature from Bob. So, rather than treating Alice's lone entry as a policy
violation, they mark Alice's entry as unverified in the local cache and proceed
Carol's entry. When Bob eventually adds his entry, the client receives it,
retrieves Alice's entry from the cache and verifies them together.

4. Alice needs to sign off on `main`. Bob needs to sign off on changes to `foo`.
Alice signs an entry for `main` with a commit ID. However, Bob points out an
issue and Alice and Bob decide on a different change instead.

```
--------------------------      -----------------      ---------------
| Alice's rejected entry | <--- | Alice's entry | <--- | Bob's entry |
--------------------------      -----------------      ---------------
```

This is fundamentally fine, the rejected entry's changes are not applied by
gittuf clients. However, the rejected entry is stored in a cache perpetually
with subsequent verification runs attempting to verify it, in case the threshold
has been met by newer entries. To avoid this, an annotation entry must be
created to mark the rejected entry as to be skipped.

## Effect of Skip Annotations

A skip annotation can mark one or more entries as invalid. If some `k` entries
of a set of `n` entries are marked as invalid in an annotation, these entries do
not count towards the threshold.

## Discussion

1. When we only have a single RSL entry for a change, a change in the repository
either has a valid entry or it doesn't. With support for multiple entries, there
exists a state where a change is made in the repository but is not considered
valid until other entries are added.

2. We have some open questions separate from this one regarding who can
authorize skip annotations for some namespace. If we limit skip annotations to
actors authorized for the ref, then Bob can't revoke his own entry as in the
above examples he's not authorized for `main`. In the original design, this
didn't happen as Bob's entry isn't considered valid in the first place. An
option may be to have revocations come from either authorized parties for the
Git ref or the author of the skipped entry.

3. How much does the order of events matter? If Alice is authorized for `main`
and Bob for `foo`, should Alice's entry come first? Can Bob sign his entry first
and Alice second?
