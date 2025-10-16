# Supporting Teams and Hats in gittuf

## Metadata

* **Number:** TBD
* **Title:** Supporting Teams and Hats in gittuf
* **Implemented:** No
* **Withdrawn/Rejected:** No
* **Sponsors:** Yongjae Chung (yongjae354)
* **Contributors:** Patrick Zielinski (patzielinski), Isabel Ayres (i-ayres),
  Peter Ye (PeterJYE)
* **Related GAPs:** [GAP-5](/docs/gaps/5/README.md)
* **Last Modified:** October 16, 2025

## Abstract
Currently gittuf doesn't have a convenient way to group multiple identities
together. In this GAP, we aim to increase the usability and scalability of
gittuf by introducing the idea of teams, where multiple identities can be
grouped together to be given equal privileges.

## Motivation
In certain scenarios, repository administrators may need to delegate
authorization to a group of people. This group may be a team of developers,
release managers, lawyers, QA, security, or etc., depending on the delegated
namespace. [GAP-5](/docs/gaps/5/README.md) introduces the notion of a principal,
which allows multiple keys to be grouped to a single entity. However, the
current implementation of principals does not support grouping multiple entities
into a single entity. This makes it difficult for a delegating entity to
identify groups within a delegation (since they are implicit), as well as to
modify all associated delegations when someone needs to be added to or removed
from a group. With teams in gittuf, the delegating entity only needs to run an
update to their team definition once, which will affect all delegations where
the team is authorized.

This GAP introduces the idea of a team in gittuf, in which multiple identities
can be grouped together as part of a single team, and a single identity can
be associated with multiple teams. 

In addition to the definition of teams, the idea of wearing different "hats"
must be supported. In some cases, people may be assigned to multiple teams, and
must be able to select which team to represent when signing off a change using
gittuf. For instance, a repository contributer Alice is a in 2 different teams:
'security' and 'dev', where these two teams have different privileges and
thresholds. In this scenario, Alice would wear her 'dev' hat during normal
updates, and wear her 'security' hat only for emergency updates that require
immediate action. As seen here, hats are a way for a person to select which team
they wish to represent when signing off a change.

## Specification
A team type contains the following fields:
* ID - A unique identifier of the team entity
* Principals - A set of principals associated with this team
* Threshold - A number of approvals required by principals associated to this
  team

The schema of a team would look like the following.
```
"<id>": {
    "teamID": "<id>"
    "principals": {
        "<personID-1>": {
            "personID": "<personID-1>",
            "publicKeys": {
                "<keyID1>": {
                    "keyid": "<keyID1>",
                    "keyid_hash_algorithms": null,
                    "keytype": "sigstore-oidc",
                    "keyval": {
                        "identity": "<username>",
                        "issuer": "<issuer>"
                    },
                    "scheme": "fulcio"
                },
                "<keyid2>>": ...
            },
            "associatedIdentities": {
                "<integrationID1>": "<usernameA>",
                "<integrationID2>": "<usernameB>",
                ...
            }
            "customMetadata": {...}
        },
        "<personID-2>": {
            ...
        },
        ...
    }
    "threshold": "<int>"
}
```

A hat attestation is used when a user wishes to approve a change by attesting on
behalf of a team. A hat authorization contains the following fields:
* TargetRef - The name of the Git reference that will be updated
* FromID - The ID of the Git reference before the change is applied.
* TargetID - The ID of the Git reference after the change is applied.
* PrincipalID - The attesting user's principal ID.
* Hat - The attesting user's team they are attesting on behalf of.

Below is an example schema of a hat attestation.
```
"refs/heads/main": {
    "targetRef": "refs/heads/main",
    "fromID": "<sha-1 hash>",
    "targetID": "<sha-1 hash>",
    "principalID": "<person-ID-1>",
    "hat": "security",
}
```

When a user attests using a hat attestation, gittuf assumes they are attesting
on behalf of a team, not as an individual person on the metadata. In other
words, a hat attestation only counts toward a team threshold, not a rule
threshold during verification. 

## Reasoning

### TODO: Verification Workflow with Teams & Hats

<!-- 
Example:
Protect-main requires 2 signatures (threshold=2). 
Authorized entities: {dev-team, Carol}
- dev-team: Principals = {Alice, Bob}, threshold = 2

Cases to consider:
- Alice signs off on her commit on main, on behalf of dev-team.
- Alice authorizes Bob's commit on main on behalf of dev-team.
- Alice authorizes Carol's change on the branch; Carol is not on dev-team, is a separate identity defined on rule.
- Carol authorizes dev-team's change on the branch; dev-team has either satisfied or not satisfied its team threshold yet.

We use attestations for both referencing git objects and authorization attestations (hat authorizations).
 -->

<!-- *TODO: We need a way to indicate that Alice is wearing dev-team hat when she
directly signs a git object. For attestations, we can use a new attestation type
`ReferenceAuthorizationWithHat` which includes hat information. However, for
direct changes, we may need to add a flag in rsl record to indicate which hat
the user is wearing. I think we should also enforce the validity of the hat the
user inputs, rather than leaving it up to the verification flow. Unless there is
a usecase for allowing temporarily invalid hat attestations in the RSL, (maybe a
user's team inclusion is approved latter than a user's hat attestation?)* -->

### TODO: Using Teams to Further Delegate Authorization
<!-- Teams can further delegate to a key, person, or another team in gittuf.  -->

## Backwards Compatibility
Introducing teams and hat selection in gittuf must be backwards compatible, as
the previous metadata definitions and verification flows of keys and principals
remain the same. Repository administrators will still be able to use
single-identity principals, and may incorporate teams in certain scenarios where
groups of collaborators must be represented. 

## Security

### Addressing Additional Key Compromise Risk
Allowing hat-based authorizations may increase the risk of key compromise for a
single user key. For example, assume a user Alice, with a high-privilege
security hat and a regular developer hat. The key compromise risk for a single
key is now higher for Alice, as obtaining Alice's private keys for developer hat
access also implies access to the security hat. A potential solution for this
problem is to require Sigstore integration for users that are members of teams,
enforcing multi-factor and ephemeral authorization.

### Counting a Team towards a Threshold
Similar to what has been discussed in [GAP-5](/docs/gaps/5/README.md), adding
teams introduces some additional complexity during verification. 

> The workflow must now track whether a key was used for another previously
examined principal to avoid counting a single signature for two principals who
both have the same key associated with them.

For team thresholds, since we don't allow for nested teams (a team consisting
another team as a member), the above workflow still applies. That is, counting
signatures towards team thresholds will still track keys and make sure they
cannot be used for two different principals.

For rule thresholds, the system must additionally track whether a key was used
for a previously examined team, to avoid counting two entities that use the same
underlying key, towards the threshold.

## Prototype Implementation

## Acknowledgements

## References
- [GAP-5](/docs/gaps/5/README.md)
- [TAP-3](https://github.com/theupdateframework/taps/blob/master/tap3.md)