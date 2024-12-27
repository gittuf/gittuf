# Authentication Evidence Attestations

Last Modified: December 18, 2024

Status: Draft

In certain workflows, it is necessary to authenticate an actor outside of the
context of gittuf. For example, later in this document is a description of a
recovery mechanism where a gittuf user must create an RSL entry on behalf of
another non-gittuf user after authenticating them. gittuf requires evidence of
this authentication to be recorded in the repository using an attestation.

## Authentication Evidence Structure

Primarily, this attestation is recorded for pushes that are not accompanied by
RSL reference entries. As such, this attestation workflow focuses on that
scenario. It has the following format:

```
TargetRef    string
FromTargetID string
ToTargetID   string
PushActor    string
EvidenceType string
Evidence     object
```

Note that this attestation's schema is a superset of the reference
authorization attestation. While that one allows for detached authorizations
for a reference update, this one is focused on providing evidence for a push.
As such, to identify the push in question, the schema consists of many of the
same fields.

The `PushActor` field identifies the actor performing the push, but did not
create an RSL entry. `EvidenceType` is a string that identifies the type of
evidence gathered. It dictates how `Evidence` must be parsed, as this field is
an opaque object that differs from one evidence type to another.

TODO: `PushActor` has this notion of tracking actors in the policy even if
they're not gittuf users. This is somewhat reasonable as this could just be a
key ID, which is used just with Git. However, we're fast approaching a
separation of actor identifier from their key ID. There's also a TAP for this
that we should look at, and think about how OIDC bits can also connect here.

TODO: Add some example evidence types for common scenarios. Push certificate
and GitHub API result (subset) ought to do the trick.

Authentication evidence attestations are stored in a directory called
`authentication-evidence` in the attestations namespace. Each attestation must
have the in-toto predicate type:
`https://gittuf.dev/authentication-evidence/v<VERSION>`.

## Using Authentication Evidence

The authentication evidence can be used to create RSL entries on behalf of other
developers. This mechanism is necessary for adoptions where a subset of
developers do not use gittuf. When they submit changes to the main copy of the
repository, they do not include RSL entries. Therefore, when a change is pushed
to a branch by a non-gittuf user A, a gittuf user B can submit an RSL entry on
their behalf. Additionally, the entry must identify the original user and
include some evidence about why B thinks the change came from A.

The evidence that the change came from A may be of several types, depending on
the context. If user B completely controls the infrastructure hosting that copy
of the repository, the evidence could be the communication of A to B that
submitted the change. For example, if A pushes to B's repository using an SSH
key associated with A, B has reasonable guarantees the change was indeed pushed
by A. Here, B may be another developer managing a "local" copy of the repository
or an online bot used by a self hosted Git server, where the bot can reason
about the communication from A. In cases where this degree of control is
unavailable, for example when using a third party forge such as GitHub, B has no
means to reason directly about A's communication with the remote repository. In
such cases, B may rely on other data to determine the push was from A, such as
the GitHub API for repository activity which logs all pushes after
authenticating the user performing the push.

Note that if A is a Git user who still signs their commits, a commit signature
signed with A's key is not sufficient to say A performed the push. Creating a
commit is distinct from pushing it to a remote repository, and can be performed
by different users. When creating an RSL entry on behalf of another user in
gittuf, the push event (which is captured in the RSL) is more important than the
commit event.
