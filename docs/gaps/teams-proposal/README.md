# Supporting Teams in gittuf

## Metadata

* **Number:** TBD
* **Title:** Supporting Teams in gittuf
* **Implemented:** No
* **Withdrawn/Rejected:** No
* **Sponsors:** Yongjae Chung (yongjae354)
* **Contributors:**
* **Related GAPs:** [GAP-5](/docs/gaps/5/README.md)
* **Last Modified:** September 22, 2025

## Abstract
Currently gittuf requires identities to be limited to a single key or person. In
this GAP, we aim to increase the usability by introducing the idea of Teams,
where multiple identities can be grouped together to be given equal privileges.

## Motivation
Often developers work in teams, and a repository administrator (or higher-level
authorized principal) may want to assign multiple entities to a single group,
delegating equal authorization to make changes on the repository.
[GAP-5](/docs/gaps/5/README.md) introduces the notion of a principal, which
allows multiple keys to be grouped to a single entity. However, the current
implementation of principals does not support grouping multiple entities into a
single entity. This GAP introduces the idea of a Team, where multiple principals
can be grouped together as part of a single entity, and a single principal can
be associated with multiple teams. 

In addition to the definition of teams, the idea of "hats" must be supported.
Developers may be included in different teams, and must be able to select which
team to represent when signing off a change. For example, a repository
contributer Alice may be part of 2 different teams: 'security' and 'dev'. These
two teams may have different privileges and thresholds.


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

TODO: add hat attestation definition, schema,
examples

## Reasoning
TODO:
- Should a team further delegating authorization downstream be allowed?

## Backwards Compatibility

## Security
TODO Allowing hat-based authorizations might increase security risk. For
example, having a principal with a high-privilege security hat and a regular
developer hat increases the key compromise risk for a single key. A potential
solution for this is to require Sigstore integration for multi-factor and
ephemeral authorization.

## Prototype Implementation

## Implementation

## Changelog

## Acknowledgements

## References
