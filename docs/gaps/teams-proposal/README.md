# Supporting Teams and Hats in gittuf

## Metadata

* **Number:** TBD
* **Title:** Supporting Teams and Hats in gittuf
* **Implemented:** No
* **Withdrawn/Rejected:** No
* **Sponsors:** Yongjae Chung (yongjae354)
* **Contributors:** Patrick Zielinski (patzielinski), Isabel Ayres (i-ayres),
  Peter Ye (PeterJYE), Reza Curtmola (reza-curtmola), Justin Cappos (
  JustinCappos)
* **Related GAPs:** [GAP-5](/docs/gaps/5/README.md)
* **Last Modified:** November 8, 2025

## Abstract

Currently, gittuf doesn't have a convenient way to group multiple persons
together and represent them as a single entity. This GAP aims to enhance the
usability of gittuf by introducing two new features. First is the new team
principal type, which represents a group of persons in the gittuf policy
metadata. Second is the support for a gittuf user to "wear a hat", allowing them
to specify a team to sign off on behalf of.

## Motivation

In certain scenarios, repository administrators may need to delegate
authorization to a group: a set of multiple users that serve the same role in a
repository. This may be a team of developers, release managers, lawyers, QA,
security, or etc. [GAP-5](/docs/gaps/5/README.md) introduces the notion of a
person principal, which allows multiple keys to be associated with a single
entity. However, the current implementation of principals does not support
grouping multiple persons into a single entity. This makes it difficult for a
delegating entity to identify a group in the system, since they are defined
implicitly (see [Reasoning](#reasoning)). Also, it is inefficient and
error-prone to modify all associated delegations when a person needs to be added
to or removed from a group. With explicit team definitions, the delegating
entity only needs to run an update to their team definition once, which will
affect all delegations where the team is authorized.

Additionally in some cases, a gittuf user may need to escalate their own
privileges under certain circumstances. For example, when a critical security
vulnerability is discovered, an emergency patch may need to be pushed directly
to the production branch although the usual process requires multiple signatures
under the branch protection rule. For urgent updates as such, a project may
benefit from a privileged 'security' team principal that can access the branch
without further approvals. To support this behavior, as well as the general
capability for a person to be assigned to multiple teams in different contexts,
we define a "hat", which specifies the team principal that the gittuf user is
signing on behalf of.

## Specification

A team type contains the following fields:

* ID - A unique name or identifier of the team entity
* Principals - A set of principals associated with this team
* Threshold - A number of approvals required by principals associated with this
  team

The schema of a team would look like the following.

```
"<id>": {
    "teamID": "<id>"
    "principals": {
        "<personID1>": {
            "personID": "<personID1>",
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
                "<keyID2>>": ...
            },
            "associatedIdentities": {
                "<integrationID1>": "<usernameA>",
                "<integrationID2>": "<usernameB>",
                ...
            }
            "customMetadata": {...}
        },
        "<personID2>": {
            ...
        },
        ...
    }
    "threshold": "<int>"
}
```

The team ID can be any arbitrary string (likely the name of the team) that
uniquely identifies the team within the policy. Note that a team threshold must
be satisfied in order for the team to count towards a rule threshold during
verification.

### Definition of a Hat

A `hat` is a specifier that indicates the intended team ID of a person, when
they sign on behalf of a team.

### Updating a Team Definition

When a team's membership or threshold must be updated, the delegating entity of
the team must update their delegation. The members of the team are not trusted
to update the team's definitions. Additionally, if a member's key must be
rotated, the delegating entity must update their delegation with the member's
key information.

### Commiting to the repository on behalf of a team

For users to indicate that they have made a change on the repository on behalf
of a team's authorization, we have two options. The implementation approach is
not decided as of now.

#### Option 1 - Encode the hat information in the commit message

When a gittuf user makes a commit to the repository on behalf of a team, they
must encode their current hat information in their Git commit message. An
example commit message may look like this:

```
Fix: Documentation
This commit fixes typos in README

hat: dev
```

This implies that during verification, gittuf must inspect the commit object
pointed by the RSL and parse the commit message to check for hat information.

#### Option 2 - Define a new attestation type for attesting a user's hat information for commits

When a gittuf user makes a commit to the repository on behalf of a team, they
must create an attestation for the hat they were wearing at the time of the
commit. We call this new attestation type a `hat attestation`. A hat attestation
structure contains the following fields.

* TargetID - The ID of the Git reference pointing to the user's Git commit.
* PrincipalID - The user's principal ID.
* Hat - The user's team ID they are committing on behalf of.

### Making an approval on behalf of a team

To allow users to make an approval on behalf of a team, we introduce an optional
field to the existing authorization attestation structure to account for hat
information. An example of an authorization attestation with hat information
looks like the following.

```
"refs/heads/main": {
    "targetRef": "refs/heads/main",
    "fromID": "<sha-1 hash>",
    "targetID": "<sha-1 hash>",
    "principalID": "<person-ID-1>",
    "hat": "security",
}
```

Here, users must specify their representing team ID in the `hat` field, which
then gittuf checks whether it is a valid team for the user.

## Reasoning

In the previous version of policy metadata, teams are implicitly defined. An
example policy metadata is shown below.

```
ruleFile: primary
principals: {Alice, Bob, Carol}
rules:
    protect-main-prod: {git:refs/heads/main,
                        git:refs/heads/prod}
        -> (2, {Alice, Bob, Carol})
    protect-ios-app: {file:ios/*}
        -> (1, {Alice})
    protect-android-app: {file:android/*}
    -> (1, {Bob})

ruleFile: protect-ios-app
principals: {Dana, George}
rules:
    authorize-ios-team: {file:ios/*}
        -> (1, {Dana, George})

ruleFile: protect-android-app
principals: {Eric, Frank}
rules:
    authorize-android-team: {file:android/*}
        -> (1, {Eric, Frank})
```

Below is the equivalent, with team prinipals defined.

```
ruleFile: primary
principals: {Alice, Bob, Carol}
rules:
    protect-main-prod: {git:refs/heads/main,
                        git:refs/heads/prod}
        -> (2, {Alice, Bob, Carol})
    protect-ios-app: {file:ios/*}
        -> (1, {Alice})
    protect-android-app: {file:android/*}
    -> (1, {Bob})

ruleFile: protect-ios-app
principals: {{ios-team: 
                principals={Dana, George}, 
                threshold=1}}
rules:
    authorize-ios-team: {file:ios/*}
        -> (1, ios-team)

ruleFile: protect-android-app
principals: {{android-team:
                principals={Eric, Frank}
                threshold=1}}
rules:
    authorize-android-team: {file:android/*}
        -> (1, android-team)
```

With team principals, a delegating entity can easily extend a rule file to
include multiple teams. For example, given the above metadata, Alice can add a
`QA-team` to the `authorize-ios-team` rule and set the rule threshold to 2,
requiring both the `ios-team` and `QA-team` to sign off.

The alternative to this approach is to either share keys between team members,
or to implicitly define them through rules. The first complicates key
management, and the latter does not allow for emergency updates, as an entity
trusted for a namespace must always be trusted. The solution to the latter
problem would be requiring users to use different keys for different contexts,
which again complicates key management, and may create [out-of-sync principal
definitions](/docs/gaps/5/README.md#out-of-sync-principal-definitions).

## Backwards Compatibility

Introducing teams and hat selection in gittuf must be backwards compatible, as
the previous metadata definitions and verification flows of keys and principals
remain the same. Repository administrators will still be able to use
single-identity principals, and may incorporate teams in certain scenarios where
groups of collaborators must be represented.

## Security

### Verification Workflow with Teams & Hats

Supporting teams and hats introduces some additional complexity during
verification. Verification must fail if a person is not wearing a valid hat. To
enforce this, gittuf must validate the person's hat worn, either at the time of
signing or retrospectively. If a hat specifier is omitted, gittuf assumes the
default hat, which specifies the person-principal itself. The alternative
default hat mechanism would be to trust the signature on all possible hats, but
this may blur the intent of the signer and the context they were signing on.

#### Counting a Team towards a Threshold

As stated in [GAP-5](/docs/gaps/5/README.md): "The workflow must now track
whether a key was used for another previously examined principal to avoid
counting a single signature for two principals who both have the same key
associated with them."

For team thresholds, since gittuf does not support nested teams (a team
consisted of another team as its member), the above workflow still applies. That
is, counting signatures towards team thresholds must still track keys and make
sure the same key cannot be used for more than 1 person on the team.

For rule thresholds, the verification flow behaves a bit more leniently. This
means that it does not explicitly track whether a person's signature was
consumed for a previously examined team, allowing itself to count more than 1
entity that use the same underlying key toward the rule threshold. Enabling this
behavior can be useful in certain cases. For example, if Bob is responsible for
both development and compliance on the branch, it may make sense to allow for
Bob to sign on behalf of both teams. gittuf ultimately leaves this decision up
to the delegating entity to decide.

### Addressing the Additional Impact of Key Compromise

Allowing hat-based authorizations may increase the impact of key compromise for
a single user key. For example, assume a user Alice with a privileged security
hat and a regular developer hat. The potential impact from a key compromise for
a single key owned by Alice is now higher, as obtaining Alice's private keys for
developer hat access also implies access to the security hat. A potential
solution for this problem is to encourage Sigstore integration for users that
are members of teams, enforcing multifactor and ephemeral authorization. The
alternative is for a user to use different keys for each hat, which reduces the
impact of a single-key compromise but would require more key management from the
user.

### Using a Team as a Delegating Entity

In some cases, it may be useful to use a team to delegate authorization to
another principal in gittuf. To achieve this, the verification workflow must
additionally check that the team thresholds are met when verifying the policy.

## Prototype Implementation

https://github.com/gittuf/gittuf/pull/1044

## References

* [GAP-5](/docs/gaps/5/README.md)
* [TAP-3](https://github.com/theupdateframework/taps/blob/master/tap3.md)
