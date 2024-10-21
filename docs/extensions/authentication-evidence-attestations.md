# Authentication Evidence Attestations

Last Modified: October 21, 2024

In certain workflows, it is necessary to authenticate an actor outside of the
context of gittuf. For example, later in this document is a description of a
recovery mechanism where a gittuf user must create an RSL entry on behalf of
another non-gittuf user after authenticating them. gittuf requires evidence of
this authentication to be recorded in the repository using an attestation.

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
