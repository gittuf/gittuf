# Code Review Tool Attestations

## Metadata

* **Number:** 6
* **Title:** Code Review Tool Attestations
* **Implemented:** No
* **Withdrawn/Rejected:** No
* **Sponsors:** Aditya Sirish A Yelgundhalli (adityasaky)
* **Related GAPs:** [GAP-3](/docs/gaps/3/README.md), [GAP-5](/docs/gaps/5/README.md)
* **Last Modified:** March 25, 2025

## Abstract

gittuf's [reference authorization
attestations](/docs/design-document.md#reference-authorization) can be used to
meet a threshold of approvals for a change in a Git repository. Here, every
approver signs the attestation themselves, minimizing the trusted entities
during verification, but this has usability concerns. To balance usability and
security, this extension proposes the use of special attestations created by
popular code review tools.

## Motivation

While gittuf supports code review approvals using reference authorization
attestations, issuing these attestations introduces significant overhead to
developers. They must now issue them using their gittuf client and sync the
attestations to the remote.

To offset this usability burden, this GAP proposes using attestations to
represent approvals performed on popular code review tools (e.g., GitHub pull
request reviews). This enables developers to continue using existing approval
workflows while also meeting gittuf policies.

## Specification

The introduction of code review tool attestations to gittuf involves changes to
the root of trust metadata schema (to bootstrap trust for the code review tool)
as well as the verification workflow (to make use of the attestation).

### Attestation Schema

The attestation is similar to the reference authorization, but also includes
information indicating the approvers as identified by the code review tool. Each
listed approver SHOULD include an immutable identifier for the approver.

```
TargetRef    string
FromID string
TargetID   string
Approvers    []string
```

The attestation must indicate the code review tool via the predicate type, and
it must be signed by the code review tool or a trusted integration with the
tool. See [verification](#verification) for how these attestations are
authenticated.

Here is an example of an attestation statement (minus the signature envelope)
for a code review approval from [@adityasaky](https://github.com/adityasaky) on
a GitHub pull request.

```json
{
    "_type": "https://in-toto.io/Statement/v1",
    "subject": [{
        "digest": {
            "gitTree": "2f70a39ab17467b112563c1bb151470ca4e51099"
        }
    }],
    "predicateType": "https://gittuf.dev/github-pull-request-approval/v0.1",
    "predicate": {
        "targetRef": "refs/heads/main",
        "fromID": "8caa1161a7d5e45122f681664ab14c8ff7c03a0e",
        "targetID": "2f70a39ab17467b112563c1bb151470ca4e51099",
        "approvers": ["adityasaky+8928778"]
    }
}
```

Here, `8928778` is the immutable ID of the approver on the code review platform.
This remains consistent even if the approver's username is changed.

### Policy

The root of trust metadata schema is extended to include the signing key of the
code review tool or platform. Specifically, the root of trust metadata includes
an additional role for code review tools. The role is associated with one or
more signing keys used by the tool to sign the attestations.

The policy schema (for rules) required for this is described in GAP-5.
Specifically, the `associatedIdentities` map is used to record the identity of
each person from the perspective of the code review tool.

### Verification

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
Specifically, in `root.json`, each trusted `integrationID` must be associated
with its signing key or identity. The attestations from the trusted apps are
loaded and their signatures are verified using the corresponding app key from
the `root.json`.

Then, the attestations's `Approvers` are used to extend the list of `personIDs`
verified via their signatures. Each approver is matched to a person using gittuf
policy, and the corresponding `personID` is added to `personIDs`. Note that
`personIDs` must contain no duplicates, to ensure the same person is not counted
twice towards the threshold. If the threshold is met with the approvers in the
code review tool attestation, then verification passes.

## Reasoning

This GAP uses the same basic semantic, that of an attestation, to track code
review approvals. The GAP also specifies how the signer for the attestation can
be trusted by the owners of a repository. As such, this approach maintains many
of gittuf's core properties (forge-agnostic record of approvals, verifiability
of source of approval claim) while improving gittuf's usability.

## Backwards Compatibility

This GAP introduces no backwards incompatibility.

## Security

The changes proposed have the following implications on gittuf's security model.

### Rotating Code Review Tool's Signing Key

After a code review tool's signing key is configured, rotating the key requires
updating the root metadata of every repository that trusts the tool's
attestations. This is a significant downside to the current approach.

### Change in Trust Model

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

## Prototype Implementation

See support for GitHub pull request approval attestations.

## References

* [GitHub pull request reviews](https://docs.github.com/en/pull-requests/collaborating-with-pull-requests/reviewing-changes-in-pull-requests/about-pull-request-reviews)
* [GitHub app](https://docs.github.com/en/apps/overview)
