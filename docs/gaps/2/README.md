# gittuf on the Forge

## Metadata

* **Number:** 2
* **Title:** gittuf on the Forge
* **Implemented:** No
* **Withdrawn/Rejected:** No
* **Sponsors:** Aditya Sirish A Yelgundhalli (adityasaky)
* **Last Modified:** January 20, 2025

## Abstract

gittuf can be deployed at the forge that all developers push changes to. This
instance of gittuf must behave differently from regular gittuf clients because
multiple developers may be pushing and fetching from the forge at any given
point in time. This GAP explores several configurations for how a gittuf-aware
forge might behave.

## Specification

TODO: The specification for this GAP depends on the configurations selected by
the community. Eventually, this GAP may be split into multiple GAPs with each
one handling a configuration that provides a subset of the desired properties
for gittuf on the forge.

## Motivation

There are several motivating factors to consider supporting gittuf on the forge:

* **Ease of deployment:** in some threat models, it may be acceptable to trust
the forge to record gittuf metadata which can be used to keep the forge honest.
In turn, these deployments are easier as lesser client-side tooling needs to be
installed and updated.
* **High traffic repositories:** for repositories with a high volume of pushes,
client-side RSL entry creation may be impractical.
* **Reject rather than recover:** in some deployments, especially with a mix of
gittuf-enabled and Git-only clients, it may be preferable to have the forge
reject bad changes rather than recover post facto to avoid serving these
unauthorized changes to Git-only clients in the interim period before a gittuf
client can initiate the recovery workflow.
* **Standardized Git security protocol:** a subset of gittuf's features can be
adopted as the standardized protocol for how forge security policies are
configured and enforced, thus enabling cross-forge validation of a repository's
historic security decisions.

## Reasoning

There are several aspects that must be considered in integrating gittuf with a
forge. These are enumerated here with a description of the default configuration
in gittuf:

* RSL Entry Creation: in the standard gittuf model, all RSL entries are created
and signed by clients when they push their changes. Thus, every push can be
authenticated using the signature on the RSL entry, and the synchronization
point is not responsible for ordering pushes in the repository meaning it cannot
reorder or drop pushes.
* Verification: in the standard gittuf model, every gittuf client performs
verification when it receives changes from the synchronization point. Typically,
this means that a change that fails verification must be fixed after the fact.
* Git Reference Updates: in the standard gittuf model, a gittuf client pushes
directly to the references on the synchronization point the user wishes to
update along with corresponding RSL entries. The RSL entries are submitted to
the synchronization point's RSL directly after the client fetches the latest
state of the RSL to ensure the new entries are added at the very end of the RSL.
The gittuf client makes this push atomically, meaning either all references are
updated or none are updated,

### Configuration A

**Summary:** Clients create RSL entries, forge performs pre-receive
verification, users update references directly.

In this configuration, users push directly to the Git references (e.g., the
branch they update and the RSL with a corresponding entry) and the forge is
integrated to perform gittuf verification at the pre-receive phase of a push.

**Pros:**
* The forge can reject pushes that fail verification, offering better
protections to Git-only clients.

**Cons:**
* The forge can carry out denial of service attacks that may or may not be
immediately obvious to the pushing actor.
* Client-side RSL entry creation can be a bottleneck for high traffic
repositories.

TODO: Should the pushing gittuf client be investigated for submitting something
that fails verification?

### Configuration B

**Summary:** Clients create RSL entries, forge performs post-receive
verification, users update references directly.

In this configuration, users push directly to the Git references (e.g., the
branch they update and the RSL with a corresponding entry) and the forge is
integrated to perform gittuf verification at the post-receive phase of a push.

**Pros:**
* The forge cannot carry out denial of service attacks beyond the freeze attacks
it can already perform.
* This configuration can be implemented in popular forges using existing
features (e.g., GitHub Actions).

**Cons:**
* The forge cannot prevent unauthorized changes from being pushed, requiring the
recovery workflow to be executed by a gittuf client after the fact.
* Client-side RSL entry creation can be a bottleneck for high traffic
repositories.

TODO: Should the pushing gittuf client be investigated for submitting something
that fails verification?

TODO: Explore making forge capable of carrying out recovery workflow. This needs
to account for race conditions with verification / recovery in high traffic
repositories.

### Configuration C

**Summary:** Forge creates pre-receive RSL entries, forge performs pre-receive
verification, users update references directly.

In this configuration, users push directly to the Git references (e.g., the
branch they update) **without** a corresponding RSL entry. The forge performs
verification at the pre-receive phase (optionally by creating a provisional RSL
entry) and rejects pushes that fail verification.  If the verification passes,
the forge makes the change available along with an RSL entry signed by it (if a
provisional RSL entry was created, this can be adopted as the final RSL entry).

**Pros:**
* The forge can reject pushes that fail verification, offering better
protections to Git-only clients.
* Deployments are simpler as client-side tooling requires fewer changes.

**Cons:**
* The forge is trusted far more than in the standard gittuf model, as it can
reorder or drop RSL entries (drops may be prevented by local "receipts",
potentially).
* With only an RSL entry for the push, there is no way to authenticate the
pushing user. If this is attested to by the forge, the forge must be trusted not
to lie.
* A malicious forge can carry out a denial of service attack by falsely claiming
verification failed.

While more trust is placed in the forge (approaching cases where the forge is
trusted solely to enforce security controls), this configuration still requires
the forge to explicitly record its decisions in the repository in a manner that
any gittuf client can verify the forge's honesty.

TODO: Must the forge attest to how it authenticated a user?

TODO: Can clients record some local-only "receipt" of a push that they validate
is in the RSL next time?

TODO: Can the forge still order pushes to handle high-traffic cases? Is a
staging area necessary?

### Configuration D

**Summary:** Forge creates post-receive RSL entries, forge performs post-receive
verification, users update references directly.

In this configuration, users push directly to the Git references (e.g., the
branch they update) **without** a corresponding RSL entry. The forge creates an
RSL entry in the post-receive phase and then performs verification.

**Pros:**
* The forge cannot carry out denial of service attacks beyond the freeze attacks
it can already perform.
* Deployments are simpler as client-side tooling requires fewer changes.
* This configuration can be implemented in popular forges using existing
features (e.g., GitHub Actions).

**Cons:**
* The forge may run into race conditions with creating RSL entries in high
traffic repositories.
* The forge is trusted far more than in the standard gittuf model, as it can
reorder or drop RSL entries (drops may be prevented by local "receipts",
potentially).
* With only an RSL entry for the push, there is no way to authenticate the
pushing user. If this is attested to by the forge, the forge must be trusted not
to lie.

TODO: Must the forge attest to how it authenticated a user?

TODO: Can clients record some local-only "receipt" of a push that they validate
is in the RSL next time?

### Configuration E

**Summary:** Forge creates pre-receive RSL entries, forge performs pre-receive
verification, users push changes to staging references.

In this configuration, users push to special Git references (e.g., a staging
area for the branch they want to update) **without** a corresponding RSL entry.
The forge performs verification at the pre-receive phase (optionally by creating
a provisional RSL entry) and rejects pushes that fail verification. If the
verification passes, the forge makes the change available along with an RSL
entry signed by it (if a provisional RSL entry was created, this can be adopted
as the final RSL entry).

**Pros:**
* The forge can reject pushes that fail verification, offering better
protections to Git-only clients.
* The forge is responsible for ordering pushes at the pre-receive phase,
simplifying RSL entry creation in high traffic repositories.
* Deployments are simpler as client-side tooling requires fewer changes.

**Cons:**
* The forge is trusted far more than in the standard gittuf model, as it can
reorder or drop RSL entries (drops may be prevented by local "receipts",
potentially).
* With only an RSL entry for the push, there is no way to authenticate the
pushing user. If this is attested to by the forge, the forge must be trusted not
to lie.
* A malicious forge can carry out a denial of service attack by falsely claiming
verification failed.

While more trust is placed in the forge (approaching cases where the forge is
trusted solely to enforce security controls), this configuration still requires
the forge to explicitly record its decisions in the repository in a manner that
any gittuf client can verify the forge's honesty.

TODO: Must the forge attest to how it authenticated a user?

TODO: Can clients record some local-only "receipt" of a push that they validate
is in the RSL next time?

TODO: Is this configuration necessary compared to C?

## Backwards Compatibility

TODO: Consider backwards compatibility after one or more configurations are
adopted.

## Security

TODO: Consider the security model of each configuration.

## Prototype Implementation

None yet.

## Changelog

* January 20th, 2025: moved from `/docs/extensions` to `/docs/gaps` as GAP-2

## References

* [Atomic Git Pushes](https://git-scm.com/docs/git-push#Documentation/git-push.txt---no-atomic)
