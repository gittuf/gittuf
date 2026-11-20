# gittuf Policy Metadata Versioning

## Metadata

* **Number:** 8
* **Title:** gittuf Policy Metadata Versioning
* **Implemented:** No
* **Withdrawn/Rejected:** No
* **Sponsors:** Aditya Sirish A Yelgundhalli (adityasaky)
* **Last Modified:** October 14, 2026

## Abstract

As gittuf is intended to be used in a distributed manner by developers
contributing to a repository, the system needs to account for version skews
between the different actors participating in a repository. Different developers
may have different versions of the client. And, different versions of the client
may support different versions of gittuf's policy schemas.

This GAP details how the gittuf implementation must handle version updates and
mismatches for a repository's policy metadata. The owners of a repository are
empowered to set the supported versions of the policy metadata, and the GAP
details how clients must behave in different scenarios involving version
mismatches. The emphasis is on ensuring that security guarantees are not
degraded while also ensuring usability for developers using older versions of
the gittuf client.

A future GAP will address versioning for attestation predicates supported in
gittuf.

## Motivation

Over time, gittuf will evolve to support new features and security guarantees.
This will lead to the creation of new schema versions for policy metadata. It is
necessary to have a systematic way to handle the introduction of new metadata
versions, or such changes may lead to degraded security guarantees (e.g., a new
security check is unenforced because an older gittuf client does not understand
this check) and poor usability.

## Specification

All gittuf policy metadata schemas must include a `SchemaVersion` field which
specifies a URI indicating the schema version. The version follows semantic
versioning. Policy metadata includes two types of files, and the type is
captured in the URI:
* For root metadata: `https://gittuf.dev/policy/root/v<MAJOR>.<MINOR>.<PATCH>`
* For rule files: `https://gittuf.dev/policy/rule-file/v<MAJOR>.<MINOR>.<PATCH>`

If the `PATCH` version is `0`, it is omitted from the URI. If the `MINOR` and
`PATCH` are both `0`, they are both omitted from the URI. The initial gittuf
policy metadata schema is unversioned, and will be treated as versions
`https://gittuf.dev/policy/root/v0.1` and
`https://gittuf.dev/policy/rule-file/v0.1`.

All gittuf policy metadata in a policy state MUST have the same schema version,
matching that of the root metadata. This prevents incompatibilities that could
arise from mixing different versions of policy metadata within the same policy
state. For instance, an older client may not be able to correctly interpret,
validate, or enforce rules specified in a rule file created with a newer schema
version.

When a new version of policy metadata is introduced, the gittuf implementation
must provide a mechanism to translate the existing policy metadata to the new
version automatically. Repository owners and other trusted parties who author
policies must not have to manually update existing policy metadata contents to
the new version (e.g., by adding all the existing rules again).  If a newer
schema version only introduces changes to one of the metadata types, the version
for the other metadata must still be updated. For instance, if the rule file
schema is updated to `v2.1` from `v2` with new fields, but the root metadata
schema remains the same as that at `v2`, the root metadata version must still be
updated to `v2.1` to maintain consistency.

### Workflows

**Update policy metadata version.** The root of trust key holders must use a
version of the gittuf client that supports the new version of the policy
metadata. The client must update all the policy metadata to the new version,
using the automated translation mechanism. If the translation fails for any
reason, the client must abort the update and notify the user of the failure.

To avoid requiring all policy metadata to be re-signed by every required
developer (e.g., for delegated rule files), the root key holders are trusted to
apply their signature to all metadata during a version update. A threshold of
root keys must be used to sign all the updated policy metadata files.

To receive a threshold of root key signatures on the updated policy metadata,
the standard staging workflow can be used. The first repository owner starts the
process by updating all the metadata files to the newer version and applying
their signature to each file. This policy state is staged. Each subsequent
repository owner reviews the staged policy metadata and adds their signature.
Once a threshold of signatures is reached for all policy metadata files, the new
policy can be applied using the standard workflow.

**Create a new policy metadata file.** The client must create a new policy
metadata file using the schema version specified by the latest root of trust
metadata. If the client supports newer versions of the policy schema, a message
may be displayed to the user. If the client does not support the policy schema
version specified by the latest root of trust metadata, the client must abort
creating the new policy metadata file, and indicate why to the user.

**Modify or sign an existing policy metadata file.** As with creating a new
policy metadata file, the client must ensure that the file being modified or
signed adheres to the schema version selected in the root metadata. If the
version is different, the client must abort and indicate the version mismatch.
The only exception is when the modification or signing operation is performed
during a version update workflow initiated by the repository owners. If the
version does match the schema version selected in the root metadata, but is
newer than the version supported by the client, the client must abort and prompt
the user to update the client.

**Verify policy validity.** The client verifies the validity of a policy state
during several operations (e.g., when applying a new policy, when loading policy
during enforcement). The root of trust metadata's signatures are verified
against the trusted root of trust keys declared in the prior policy state. Each
policy state is also checked for internal consistency to ensure all metadata
files are signed by the required threshold of keys (e.g., a delegated rule file
must be signed by a threshold of principals declared in the parent rule file
that delegates to it).

This validation is modified to allow any policy metadata file to be signed by a
threshold of root of trust keys, rather than just the parties directly
responsible for that policy metadata. Note that a combination of principals from
the root of trust set and the parties directly responsible for the policy
metadata cannot satisfy the required threshold. The metadata must be signed by
either a threshold of root of trust keys or a threshold of the parties directly
responsible for that policy metadata.

Finally, the validation workflow must also check that all the policy metadata
files in a policy state are of the same schema version, as defined in the root
of trust metadata. If the client detects a mismatch or does not support the
schema version, it must abort.

## Reasoning

This GAP considers different versioning strategies and their tradeoffs. The key
difference between the strategies is the entity responsible for selecting the
metadata versions used in a repository.

### Versioning Strategies

**Repository owners select versions.** In this strategy, the owners of a
repository (i.e., the root key holders) determine the gittuf policy metadata
version used in the repository at any point in time. This is the strategy
adopted in this GAP's [specification](#specification). This approach ensures
that the repository, at any point in time, has a consistent set of metadata
versions, i.e., a policy state cannot have mismatched metadata versions.

In addition, this approach allows repository owners to determine when the
repository should upgrade to newer versions. This allows them to plan for such
upgrades, such as by informing developers contributing to the repository so they
have a window of time to update their clients.

**Clients select versions.** In this strategy, the client determines the
versions of metadata used in a repository. For example, when creating a new rule
file, the client may use the latest version it supports, which may be newer than the
version used for other policy metadata in the repository.

This approach introduces several drawbacks. A repository may have multiple
clients, only a subset of which support the latest schema versions. All clients
are forced to maintain an upgrade cadence on par with the newest client
operating on the repository. In addition, this approach can lead to inconsistent
schema versions within a single policy state, which can lead to gaps in
validation of the policy itself and inconsistencies in enforcement.

### On Semantic Versioning

Semantic versioning allows for communicating compatibility between different
versions. However, the current [specification](#specification) does not make use
of the compatibility information provided by semantic versioning. For example,
there is no allowance for a "patch" schema update to be applied to a subset of
the repository's policy metadata files. While this GAP does not provide these
capabilities, it keeps the possibility open for future enhancements that could
leverage semantic versioning for more granular compatibility checks. In the
meantime, semantic versioning is still a useful convention for the gittuf
maintainers themselves to reason about the impact of changes to policy metadata
over time.

## Backwards Compatibility

This GAP does not have impact on backwards compatibility of the existing gittuf
design and implementation. Indeed, the GAP introduces granular versioning of
metadata and behavior, which impacts how gittuf handles backwards compatibility
going forward.

## Security

The GAP considers how policy metadata is versioned. It empowers root of trust
key holders to make decisions regarding the versions of policy metadata used in
the repository. The GAP also considers how clients should behave when they
encounter policy metadata of an unsupported or unexpected versions.

**Why must all metadata versions be consistent within a state?** The
[specification](#specification) requires all metadata versions to be consistent
within a state. This reduces complexity for the client. If metadata versions are
inconsistent, a client may understand only a subset of the policy metadata. This
can introduce errors in policy enforcement (e.g., a newer client that
understands all of the metadata may block a change allowed by an older client
that does not understand the newer metadata). Inconsistent versions may also
have incompatible expectations that are not caught in time, leading to
unexpected behavior.

**Why are repository owners allowed to always sign all policy metadata?** The
[specification](#specification) updates policy state validation to allow
repository owners to sign all policy metadata, **even when the change is not a
version update**. Allowing this during version updates is reasonable from a
usability perspective, as the alternative would require every metadata file to
be re-signed by the parties directly trusted to do so, at all levels of the
delegation tree. Allowing this otherwise may, however, seem overly permissive.

But, this does not grant repository owners powers beyond what they already
possess. The repository owners are the ultimate authority over the repository
and its policy metadata. They can already make changes to policy metadata,
beyond just the root of trust metadata. For instance, they can grant themselves
authority over the primary rule file all the way down to any delegated rule
file, effectively allowing them to sign all policy metadata if they choose to do
so. On the other hand, by making this explicit, the implementation becomes
simpler--it is not necessary to separately handle the verification of version
updates from other policy changes to determine if root key signatures are
trusted for policy metadata.

## Prototype Implementation

Some of the changes proposed in this GAP such as policy metadata versioning have
been added to the gittuf implementation.
