# gittuf on the Forge

## Metadata

* **Number:** 2
* **Title:** gittuf on the Forge
* **Implemented:** No
* **Withdrawn/Rejected:** No
* **Sponsors:** Aditya Sirish A Yelgundhalli (adityasaky), Paulo Gomes (pjbgf)
* **Last Modified:** August 11, 2026

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
reorder or drop pushes. When the forge creates RSL entries (Configurations C to
F), it may enrich them with custom fields (e.g., a stable repository identifier or
an attestation of how it authenticated the pushing user). Such fields are advisory
at this point in time.
* Verification: in the standard gittuf model, every gittuf client performs
verification when it receives changes from the synchronization point. Typically,
this means that a change that fails verification must be fixed after the fact.
Verification is not limited to clients. The forge can verify entries as well, and
in some configurations it does so before persisting them. Where the forge
verifies at the pre-receive phase, it can enforce that only valid and policy
abiding entries are ever persisted, rejecting the rest instead of relying on a
client to detect and repair them later.
* Git Reference Updates: in the standard gittuf model, a gittuf client pushes
directly to the references on the synchronization point the user wishes to
update along with corresponding RSL entries. The RSL entries are submitted to
the synchronization point's RSL directly after the client fetches the latest
state of the RSL to ensure the new entries are added at the very end of the RSL.
The gittuf client makes this push atomically, meaning either all references are
updated or none are updated.
* Root-of-Trust Ownership: in the standard gittuf model the root of trust
(`refs/gittuf/policy`) is authored and signed by repository owners on the client,
and the synchronization point only stores it. A forge may instead own the root of
trust: it authors and signs root metadata and policy, and clients treat
`refs/gittuf/policy` as read-only. To avoid locking a repository to a single
forge, a forge that owns the root of trust must either co-own it with the
repository owner (the owner retains an authorized signing key), or provide a
documented takeover path so the owner can reclaim sole authority over the root
and migrate to another forge. This changes who must be trusted for authorization
decisions.

### Reference namespace and push semantics

Independent of configuration, a gittuf-aware forge should maintain one invariant
over the reference namespace: the set of references it hides from advertisement
and the set it rejects on push are the same set. A reference is either a user or
verification surface (advertised and, where applicable, writable) or it is
internal or client-local (hidden and unwritable). Nothing is hidden-but-writable
or advertised-but-reserved. In particular, `refs/local/gittuf/**` is client-local
and must be both rejected on push and unadvertised on fetch. A push that updates
more than one gittuf reference (`refs/gittuf/policy`, `refs/gittuf/policy-staging`,
`refs/gittuf/reference-state-log`, `refs/gittuf/attestations`) should be applied
atomically, since a policy change and its RSL entry form one logical trust
transaction.

Which of these references are client-writable and advertised depends on the
configuration. In configurations where clients own the root of trust,
`refs/gittuf/policy-staging` is a shared, advertised reference that maintainers
push to in order to collaboratively author and sign policy before it is promoted
to `refs/gittuf/policy`. Where the forge owns the root of trust, that same
reference is internal to the forge (see Configuration F).

This invariant also bounds what the forge signs. When the forge creates RSL
entries, it signs an entry for a reference only if that reference is
client-visible, with the RSL reference itself as the single carve-out since it
cannot record itself. Signing for a hidden reference would defeat the invariant
and could leak internal or client-local reference spaces to anyone verifying the
RSL.

A mixed push updates both gittuf references and user references in one push. As an
initial approach, mixed pushes need not be atomic across the two classes. The
gittuf reference updates are kept regardless of whether the user references pass
verification. User references that fail are rejected, and the gittuf updates in
the same push are retained. This keeps the trust state advancing on its own terms
and lets a client retry the content without re-authoring metadata. The non-atomic
phase must preserve one invariant. A retained gittuf update must be independently
valid on its own, so that a rejected user reference never leaves dangling or
unverifiable metadata behind. Full atomicity, where a mixed push either lands
entirely or not at all, is the intended end state.

TODO: Specify the exact commit protocol for fully atomic mixed pushes, and whether
these namespace rules are normative for all configurations.

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

### Configuration F

**Summary:** Forge owns the root of trust, forge creates and signs RSL entries,
forge performs pre-receive verification, users update references directly.

In this configuration the forge authors `refs/gittuf/policy` (root metadata and
policy) and signs it with a server-held root key, and it signs RSL entries with a
server-held RSL key that the policy authorizes. Users push ordinary Git
references (e.g., the branch they update) **without** a corresponding RSL entry
and treat `refs/gittuf/**` as read-only. The forge verifies at the pre-receive
phase, rejects pushes that fail, and creates the RSL entry for those that pass.
The RSL and applied policy remain fetch-visible so any gittuf client can verify
the repository against the forge's published keys.

Because the forge authors policy here, `refs/gittuf/policy-staging` is internal to
the forge rather than a collaboration point. Clients do not push it, so pushes to
it are rejected, and the forge need not advertise it. This is the opposite of the
configurations where clients own the root of trust and stage policy through that
reference.

This extends Configuration C. In addition to creating RSL entries, the forge
owns the trust chain those entries are verified against. Because
owning the root of trust can lock a repository to one forge, this configuration
requires the forge to either co-own the root with the repository owner or offer a
takeover path so the owner can reclaim it and migrate (see Root-of-Trust
Ownership above).

The root of trust may be defined fleet-wide, with a small set of server-held
signing keys anchoring many repositories at once. The forge should observe good
security hygiene and enforce segregation of duties, keeping the root-signing key
and the RSL-signing key distinct so that a compromise of one does not become a
compromise of the other.

**Pros:**
* The forge can make every hosted repository verifiable with no client-side
gittuf tooling and no bootstrap ceremony, carrying GAP-2's ease-of-deployment
motivation through to trust setup.
* Because the forge owns policy, it can guarantee every advertised reference is
covered by a verifiable chain, giving Git-only clients the "reject rather than
recover" property without requiring gittuf clients to drive recovery.
* A small set of server-held keys with defined rotation replaces per-repository
owner key management, which matters at fleet scale.

**Cons:**
* The forge is trusted for authorization and not only for ordering. It can author
any policy and sign any RSL entry. This is the strongest trust assumption in
GAP-2's spectrum. It is bounded, though not eliminated, by out-of-band publication
of the forge's keys and tiered metadata expiry (defined by TUF but not yet
implemented in gittuf).
* Owning the root of trust risks locking a repository to the forge unless
co-ownership or a takeover path is provided.
* First contact is trust-on-first-use unless an out-of-band anchor (an
owner-published root hash, a key-transparency log, or a pinned key set shipped
with tooling) is layered on top.
* When the root is shared fleet-wide, root-key loss or compromise is a
fleet-scale event. Per-repository roots reduce that blast radius, but at
server-held custody the forge holds every root key regardless of scope, so
containment is limited unless root custody is offline.

TODO: Where does the repository-identity binding live, and can gittuf make a
custom RSL field verification-relevant rather than merely tolerated?

TODO: Define the exact protocol for co-ownership or owner takeover of a
forge-managed root of trust.

## Backwards Compatibility

Configuration F is additive: it introduces a new configuration, a root-of-trust
ownership axis, and namespace guidance without changing the meaning of A to E. The
RSL byte format, policy metadata format, and reference names are unchanged, so a
client verifying a Configuration-F repository uses the standard gittuf verify
path against the forge-published root with no client-side format change. Custom
RSL fields are forward-compatible, as existing clients tolerate unknown fields.

Because the format is unchanged, a forge can start with Configuration F and later
transition toward the more client-driven configurations without a break. Root of
trust, policy signing, and attestation can each be co-shared with users
incrementally, handing authority back one responsibility at a time as clients
adopt gittuf tooling, until the repository reaches whichever split of duties its
owners want.

This transition spans a wide range of intermediate options. At one end the server
owns everything as in Configuration F. Moving along the range, the root of trust
can be handed back to users entirely, removing it from the server, while the
server is still granted a narrower role in policy for the operations the forge
wants to enforce (e.g., policy may require the server to sign or attest a given
operation such as an RSL entry). In these cases the server retains no root
authority but remains an authorized signer for its assigned role, and it can
reject any push whose policy change would remove the server from that role. The
forge thereby keeps the guarantees it must enforce without owning the trust
chain.

TODO: Consider backwards compatibility after one or more configurations are
adopted.

## Security

Configuration F sits at the far end of GAP-2's trust spectrum. The forge is
trusted for authorization and not only for ordering. This trust is bounded,
though not eliminated, by:

* Out-of-band publication of the forge's signing keys, so a rogue key is
detectable.
* Tiered metadata expiry, so a frozen or compromised forge cannot serve
stale-but-signed trust indefinitely. Metadata expiry is defined by TUF, where each
metadata role carries its own expiration, but gittuf does not yet implement it, so
until it does this is an aspiration.
* Separation of the root-signing and RSL-signing keys, so a hot-key compromise
cannot rewrite policy or root.

Anti-transplant binding: because one server-held RSL key may be authorized across
many repositories, a validly-signed entry is cryptographically valid in every
repository that trusts that key. Genesis and backfill entries could be identically
shaped across repositories. Such a forge needs the signed entry to carry a
stable, globally-unique repository identifier that the verifier checks against a
root-pinned value. This applies to any forge that reuses one RSL key across
repositories (C, D, E, and F alike).

Non-fast-forward protection: every path that can move a `refs/gittuf/**`
reference should only advance it to a descendant of the current tip. A
non-fast-forward move of policy or the RSL is a trust reset, not a write, and
should require an explicit operator-driven procedure rather than being reachable
through an ordinary push.

TODO: Should GAP-2 require metadata expiry enforcement as a precondition for
advertising any forge-trusted configuration as offering a freshness guarantee?

TODO: Consider the security model of Configurations A to E in the same terms.

## Prototype Implementation

None yet.

## Changelog

* January 20th, 2025: moved from `/docs/extensions` to `/docs/gaps` as GAP-2
* August 11th, 2026: added Configuration F (server-managed root of trust and RSL
signing with pre-receive verification), the root-of-trust ownership axis, and RSL
custom-field enrichment. Added a "Reference namespace and push semantics" section
covering the hidden-equals-rejected invariant, server-managed and client-local
reference classes, the signer authority boundary, and mixed-push handling.
Expanded the Verification aspect to cover forge-side verification. Documented the
transition spectrum from Configuration F toward client-driven configurations.
Filled the Security and Backwards Compatibility sections

## References

* [Atomic Git Pushes](https://git-scm.com/docs/git-push#Documentation/git-push.txt---no-atomic)
