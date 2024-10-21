# Principals, not Keys

## Metadata

* **Number:** 5
* **Title:** Principals, not Keys
* **Implemented:** No
* **Withdrawn/Rejected:** No
* **Sponsors:** Aditya Sirish A Yelgundhalli (adityasaky)
* **Last Modified:** March 25, 2025

## Abstract

gittuf's policy, like TUF metadata that it is inspired by, assumes that all
actors are [represented by their signing keys](/docs/design-document.md#actors).
This assumption leads to usability problems in Git development in contrast to
traditional TUF deployments in packaging ecosystems and the like.

## Motivation

There are more contexts where a developer may sign in a Git repository protected
by gittuf compared to a standard TUF deployment. Developers may contribute code
from different devices or cloud based IDEs that have distinct signing keys. The
current gittuf policy would represent these different signing keys as
independent actors, making policy misconfigurations that allow a developer to
meet a threshold using multiple keys they control more likely.

Additionally, as gittuf may integrate multiple systems (a code forge like
GitHub, a code review tool like Gerrit), the policy ought to be able to track a
developer's identity across all of these systems. Doing so would reduce
cognitive overload for maintainers who manage policies, and would enable
integrating into metadata such as in-toto attestations produced by these other
systems.

## Specification

This GAP introduces the notion of a generic "principal". A principal can be a
single key or a more complex entity that contains more than one signing key and
other custom metadata. A principal has the following attributes:
* ID: A unique identifier for the principal
* Keys: A set of public keys that can be used to verify signatures, no
  duplicates are allowed
* CustomMetadata: Additional custom metadata that must be associated with the
  principal

Additionally, this GAP introduces the first new principal type, to represent a
"person". A "person" has the following schema:

```
"<id>": {
    "personID": "<id>",
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
}
```

Here, `personID` can be set to any unique (within the policy) identifier of a
person such as their name or email address, similar to
[TAP-12](https://github.com/theupdateframework/taps/blob/master/tap12.md). Each
key associated with a person follows the schema already used by keys in gittuf's
policy today. `associatedIdentities` records the person's identity on
integrations. Finally, `customMetadata` allows for recording arbitrary
attributes with each person for further extensibility. To meet the generic
principal schema, the person type considers a combination of `customMetadata`
and `associatedIdentities` as the principal's custom metadata.

Here is an example person using this schema:

```
"aditya@example.com": {
    "personID": "aditya@example.com",
    "publicKeys": {
        "B83110D012545604": {
            "keyid": "B83110D012545604",
            "keytype": "gpg",
            "keyval": {
                "public": "<pubkey>"
            },
            "scheme": "gpg"
        },
        "aditya@example.com::https://github.com/login/oauth": {
            "keyid": "aditya@example.com::https://github.com/login/oauth",
            "keytype": "sigstore-oidc",
            "keyval": {
                "identity": "aditya@example.com",
                "issuer": "https://github.com/login/oauth"
            },
            "scheme": "fulcio"
        },
    },
    "associatedIdentities": {
        "https://github.com": "adityasaky+8928778",
    }
    "customMetadata": {}
}
```

Each associated identity SHOULD be recorded with an immutable value. For
example, some source control platforms allow a user to change their username,
with the old username up for grabs for another user to claim. Here, the username
alone is insufficient to identify the user uniquely. In the example above,
`8928778` is a unique identifier on github.com that does not change even if the
username is updated. For readability, the associated identity includes both the
usename and the unchanging ID, with only the latter ensuring uniqueness.

### Updating Principal Definitions

When dealing directly with keys, a delegating entity must update their
delegation when a developer must rotate their key. With principals, this remains
the same. If `Alice` delegates to the `Bob` person in a delegation, if Bob
updates his keys, then `Alice` must update her definition for `Bob`.

See [Security](#security) for considerations with frequent updates to principal
definitions.

### Using Principal Keys For Verification

During the verification workflow, each principal trusted for the specified path
must be examined to see if they are to be counted towards the threshold. This is
very similar to the existing verification workflow where each key is examined,
and is in fact identical when the principal is a single key.

For multiple-key principals, every key associated with the principal must be
used to verify the signature. Even if the principal has issued multiple
signatures using their keys, they must only be counted once towards the
threshold.

While the keys associated with a principal are unique, it's possible that
multiple principals have the same key associated with them. For example, due to
a misconfiguration, the policy may indicate that both Bob and Carol use key
`abcdef`. A signature from this key MUST only count towards the threshold once.
If the key was encountered via Bob, then in addition to counting Bob towards the
threshold, the key must also be marked as used, so it's not trusted when the
verification workflow checks Carol's keys.

## Reasoning

As noted in the [Motivation](#motivation), gittuf's deployment scenarios
necessitate knowing more about a developer to support flexibility. The approach
employed in this GAP is to support grouping multiple keys as a single actor, as
well as associating other metadata with the actor. The generic principal type
can be expanded in other ways: for now, it supports single keys and persons.
However, this can be expanded to support other special entities such as bots.
The principal type can also be used to represent a group of principals. For
example, a team principal type can be defined to represent a group of
developers, each defined using a person principal type.

While this proposal introduces some complexity in gittuf's verification
workflows (see [Security](#security)), the alternative is to move the complexity
to repository maintainers who author policies, and have to reason about how to
handle multiple keys for a single developer.

## Backwards Compatibility

The new schema is not backwards compatible with the key-based approach in the
gittuf design. This must be introduced using a new version of the policy
metadata schema. Additionally, the gittuf client must continue supporting
existing policies that use keys directly.

## Security

This GAP introduces some concepts that have security implications.

### Out of Sync Principal Definitions

The delegating entity is responsible for associating a unique identifier for the
delegatee "person". While this is unique within the context of a delegated file,
other delegated files may use the same ID for another person. To avoid policy
misconfiguration or confusion issues, the person definitions SHOULD be
consistent across metadata. In other words, the definition for `Bob` SHOULD be
the same when delegated to by `Alice` in role `foo` and when delegated to by
`Carol` in role `bar`. This introduces a usability issue: when `Bob`'s
definition must be changed, all delegations must be updated as well. While this
is also the case for when a key must be rotated or revoked, storing additional
metadata for each person makes it likely that changes are more frequent.

A possible solution is to centralize (within gittuf metadata) all person
definitions. This can be achieved with a dedicated role for person declarations,
delegated to by the repository's root of trust. This role becomes the source of
truth for all person definitions in the repository's rule files. The downside of
this solution is that a developer cannot delegate to an entirely new person
anymore, though this may be a desirable feature in some contexts. For example,
in an enterprise context, there may be a desire to allow delegations but limit
them to trusted developers part of the company.

Centralizing person declarations could also enable each developer to manage
their own `person` definition. The dedicated role for definitions can be sharded
on a per-person basis, allowing each person to manage their own keys and
associated identities.

TODO: does self managed mean a developer can add themselves? Claiming an ID is
complicated. Being too prescriptive is also not a good idea about how the ID
ownership is verified.

### Counting a Principal towards Threshold

This GAP introduces some complexity in the verification workflow, specifically
with how a principal is counted towards the threshold. The workflow must now
track whether a key was used for another previously examined principal to avoid
counting a single signature for two principals who both have the same key
associated with them.

However, this complexity is acceptable given the alternative. Without the
ability to associate multiple keys with a single principal, the maintainers
responsible for declaring policies have to write unintuitive rules, and
misconfigurations that allow a single developer to be counted multiple times
towards the threshold are likelier.

## Prototype Implementation

See v0.2 policy metadata. To use this metadata, set `GITTUF_DEV=1` and
`GITTUF_ALLOW_V02_POLICY=1`.

## References

* [TAP-12](https://github.com/theupdateframework/taps/blob/master/tap12.md)
