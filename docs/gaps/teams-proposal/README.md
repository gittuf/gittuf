# Supporting Teams and Hats in gittuf

## Metadata

* **Number:** TBD
* **Title:** Supporting Teams in gittuf
* **Implemented:** No
* **Withdrawn/Rejected:** No
* **Sponsors:** Yongjae Chung (yongjae354)
* **Contributors:**
* **Related GAPs:** [GAP-5](/docs/gaps/5/README.md)
* **Last Modified:** September 23, 2025

## Abstract
Currently gittuf requires identities to be limited to a single key or person. In
this GAP, we aim to increase the usability by introducing the idea of Teams,
where multiple identities can be grouped together to be given equal privileges.

## Motivation
In some usecases of gittuf, repository administrators may need to delegate
authorization to a group of people. This group may be a team of developers,
release managers, lawyers, QA, security, or etc., depending on the delegated
namespace. [GAP-5](/docs/gaps/5/README.md) introduces the notion of a principal,
which allows multiple keys to be grouped to a single entity. However, the
current implementation of principals does not support grouping multiple entities
into a single entity. This makes it difficult for a delegating entity to
identify groups within a delegation (since they are implicit), as well as
modifying all associated delegations when someone needs to be added to or
removed from a group.

This GAP introduces the idea of a Team in gittuf, in which multiple principals
can be grouped together as part of a single entity, and a single principal can
be associated with multiple teams. In addition to the definition of teams, the
idea of "Hats" must be supported. some people may be assigned to multiple teams,
and must be able to select which team to represent when signing off a change
using gittuf. For example, a repository contributer Alice may be part of 2
different teams: 'security' and 'dev'. These two teams may have different
privileges and thresholds.


## Specification
A Team type contains the following fields:
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

TODO: add hat attestation definition, schema, examples
- hat attestation should only count towards a team threshold, not a general rule
  threshold.
    - if a user attests using a hat, we assume they are representing a team.
- during current verification workflow, there is a part where we count all
  principals towards a threshold. 
- we expand that area to include teams as well, generalizing the current flow to
  entities, not principals.
- for a team, we check if there are hat attestations from members and it fills
  the threshold requirement.
- we need to track keys used for those team members, so that it is not included
  in the future for individual approvals.

## Reasoning
TODO:
- Should a team further delegating authorization downstream be allowed?
- Add changes to verification workflow

## Backwards Compatibility
Introducing teams and hat selection in gittuf must be backwards compatible, as
the previous metadata definitions and verification flows of keys and principals
remain the same. It is up to the repository administrators to choose between
using previous key-based or principal-based metadata for smaller projects, and
incorporating teams in larger projects with groups of collaborators. 

## Security
TODO: Allowing hat-based authorizations may increase key compromise risk. For
example, having a principal with a high-privilege security hat and a regular
developer hat increases the key compromise risk for a single key. A potential
solution for this is to require Sigstore integration for multi-factor and
ephemeral authorization.

### Counting a Team towards a Threshold
Similarly to what has been discussed in [GAP-5](/docs/gaps/5/README.md), adding
teams introduces some additional complexity during verification. 

> The workflow must now track whether a key was used for another previously
examined principal to avoid counting a single signature for two principals who
both have the same key associated with them. [GAP-5, Counting a Principal towards a
Threshold](/docs/gaps/5/README.md##Security###Counting-a-Principal-towards-Threshold)

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