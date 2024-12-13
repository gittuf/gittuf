# Persons, not Keys

Last Modified: October 31, 2024

Status: Draft

gittuf's policy, like TUF metadata that it is inspired by, assumes that all
actors are [represented by their signing keys](/docs/design-document.md#actors).
This assumption leads to usability problems in Git development in contrast to
traditional TUF deployments in packaging ecosystems and the like.

There are more contexts where a developer may sign in a Git repository protected
by gittuf compared to a TUF deployment. Developers may contribute code from
different devices or cloud based IDEs that have distinct signing keys. The
current gittuf policy would represent these different signing keys as
independent actors, making policy misconfigurations that allow a developer to
meet a threshold using multiple keys they control more likely.

Additionally, as gittuf may integrate multiple systems (a code forge like
GitHub, a code review tool like Gerrit), the policy ought to be able to track a
developer's identity across all of these systems. Doing so would reduce
cognitive overload for maintainers who manage policies, and would enable
integrating into metadata such as in-toto attestations produced by these other
systems.

Generalizing these requirements (multiple keys for a developer, extended
attributes for a developer), gittuf policy must be updated to recognize the
notion of a "person" using the following schema:

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
[TAP-12](https://github.com/theupdateframework/taps/blob/master/tap12.md).  Each
key associated with a person follows the schema already used by keys in gittuf's
policy today. `associatedIdentities` records the person's identity on
integrations. Finally, `customMetadata` allows for recording arbitrary
attributes with each person for further extensibility.

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

## Managing Person Definitions

When dealing directly with keys, a delegating entity must update their
delegation when a developer must rotate their key. With persons, this remains
the same. If `Alice` delegates to the `Bob` person in a delegation, if Bob
updates his keys, then `Alice` must update her definition for `Bob`.

This also allows Alice to define the unique ID for Bob's person definition.
While this is unique within the context of a delegated file, other delegated
files may use the same ID for another person. To avoid policy misconfiguration
or confusion issues, the person definitions must be consistent across metadata.
In other words, the definition for `Bob` must be the same when delegated to by
`Alice` in role `foo` and when delegated to by `Carol` in role `bar`. This
introduces a usability issue: when `Bob`'s definition must be changed, all
delegations must be updated as well. While this is also the case for when a key
must be rotated or revoked, storing additional metadata for each person makes it
likely that changes are more frequent.

## Centralizing Person Declarations

A possible solution to centralize (within gittuf metadata) all person
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
