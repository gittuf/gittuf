# Code Review Tool Attestations

Last Modified: October 22, 2024

gittuf's [reference authorization
attestations](/docs/design-document.md#reference-authorization) can be used to
meet a threshold of approvals for a change in a Git repository. Here, every
approver signs the attestation themselves, minimizing the trusted entities
during verification, but this has usability concerns. To balance usability and
security, this extension proposes the use of special attestations created by
popular code review tools.

## Attestation Schema

The attestation is similar to the reference authorization, but also includes
information indicating the approvers.

```
TargetRef    string
FromTargetID string
ToTargetID   string
Approvers    []string
```

The attestation must indicate the code review tool via the predicate type, and
it must be signed by the code review tool or a trusted integration with the
tool. See [verification](#verification) for how these attestations are
authenticated.

## Policy

gittuf policy must be updated to extend the information tracked for each trusted
person. Currently, each trusted person is represented directly by their public
key in policy metadata, similar to how TUF represents actors. To support code
review tool attestations, this must be extended to support the identity of an
approver from the perspective of the code review tool. For example, Alice may
use an SSH key for commits and RSL entries but use GitHub's cloud offering for
code review. Thus, gittuf policy must associate her public key and her GitHub
username / identifier. Also see: https://github.com/gittuf/gittuf/issues/586.

This extension proposes the following format for the policy schema:

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
    "associatedAppIdentities": {
        "<appID1>": "<usernameA>",
        "<appID2>": "<usernameB>",
        ...
    }
    "customMetadata": {...}
}
```

This schema supports associating multiple code review tools (via
`associatedAppIdentities`) with a single person. In addition, it also supports
associating multiple public keys with a person, which simplifies creating
policies that can ensure only one of a person's keys are trusted to meet a
rule's threshold.

## Verification

The verification workflow follows the [default
workflow](/docs/design-document.md#verification-workflow) until the reference
authorization is verified. If the threshold is met, then verification succeeds
without using the code review tool attestations. If the threshold is not met,
the verification workflow moves on to using the code review tool attestations.
Note that during verification, the set of `personIDs` verified using reference
authorizations is tracked even if the threshold is not met to ensure that the
same person is not also trusted via a code review tool approval.

If a threshold is not met, then code review tool approvals are used for the
change in question. The approval attestations are loaded for apps or tools that
are explicitly marked as trusted in the repository's root of trust metadata.
Specifically, in `root.json`, each trusted `appID` must be associated with its
signing key or identity. The attestations from the trusted apps are loaded and
their signatures are verified using the corresponding app key from the
`root.json`.

Then, the app's `Approvers` are used to extend the list of `personIDs` verified
via their signatures. Each approver is matched to a person using gittuf policy,
and the corresponding `personID` is added to `personIDs`. Note that `personIDs`
must contain no duplicates, to ensure the same person is not counted twice
towards the threshold. If the threshold is met with the approvers in the code
review tool attestation, then verification passes.

## Change in Trust Model

Using code review tools or apps reintroduces single points of trust to gittuf
verification. The app or tool can indicate an approval happened when the person
in question did not actually issue an approval. One aspect of the attestations,
however, is that it introduces non-repudiation: the code review tool cannot
indicate a change is approved by a person and later claim otherwise, as the
attestation cannot be amended without violating the RSL's append-only property.

Another possible mitigation is to have thresholds for these attestations. This
may be possible when the attestation issuer is some other integration into a
code review tool (e.g., a [GitHub app](https://docs.github.com/en/apps/overview)
that watches GitHub pull requests), where multiple, isolated integrations can be
used. However, if the issuer is the code review tool itself, then we cannot
employ thresholds.
