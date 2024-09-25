# gittuf Versioning Policy

## Metadata

* **Number:** TBD
* **Title:** gittuf Versioning Policy
* **Implemented:** No
* **Withdrawn/Rejected:** No
* **Sponsors:** Aditya Sirish A Yelgundhalli (adityasaky)
* **Last Modified:** December 31, 2025

## Abstract

As gittuf is intended to be used in a distributed manner by developers
contributing to a repository, we need to have a plan in place for version skews.
It will inevitably be the case that different developers have different versions
of the client. Also, different versions of the client may support different
versions of gittuf's policy and attestation schemas. If not handled with care,
this can lead to unintended consequences, including degraded security
guarantees.

This GAP details how the gittuf implementation handles version updates and
mismatches. The owners of a repository are empowered to set the supported
versions of the policy metadata and attestation predicates, and the GAP details
how clients must behave in different scenarios involving version mismatches.
The emphasis is on ensuring that security guarantees are not degraded while also
ensuring usability for developers using older versions of the gittuf client.

## Motivation

Over time, gittuf will evolve to support new features and security guarantees.
This will lead to the creation of new schema versions for policy metadata and
attestation predicates. It is necessary to have a systematic way to handle the
introduction of new metadata versions, or such changes may lead to degraded
security guarantees (e.g., a gittuf client fails to enforce new security checks
it cannot understand) and poor usability.  The aim of this GAP is to assess
different versioning strategies and settle on one that sufficiently balances
security and usability.

## Reasoning

This GAP considers different versioning strategies and their tradeoffs. The key
difference between the strategies is the entity responsible for selecting the
metadata versions used in a repository.

### Repository Owners Select Versions

In this strategy, the owners of a repository (i.e., the root key holders)
determine the gittuf metadata versions used in the repository at any point in
time. This is the strategy adopted in this GAP's
[specification](#specification). This approach ensures that the repository, at
any point in time, has a consisent set of metadata versions, i.e., a policy
state cannot have mismatched metadata versions. This prevents potential security
gaps that may be caused by mixing different versions (e.g., a policy metadata
file relies on a security guarantee introduced in another file in its version,
but the corresponding file is of an older version that does not have the
guarantee). In addition, this approach allows repository owners to determine
when the repository should upgrade to newer versions, allowing them to plan for
such upgrades (e.g., informing developers contributing to the repository to
update their clients).

#### Why must a client abort for unsupported newer versions?

In this approach, the [specification](#specification) requires clients to abort
when they encounter a newer version of metadata they do not support.  This is
important because while this may be inconvenient for the user, it prevents
security guarantees from being degraded. For example, if a client that does not
support a newer version of policy metadata is used for verification, not
aborting means that the client may allow changes that would have been prevented
by a client that understands the newer version of the policy metadata.

#### Why must a client only warn when using older versions?

In this approach, the [specification](#specification) requires clients to only
print warnings when they encounter older versions of metadata than they support.
This is important to ensure users are aware of newer versions that may have
improved security guarantees. However, the client must not abort the operation
as this would lead to poor usability. For example, if a client encounters an
older version of policy metadata during verification, aborting would prevent
verification of historical repository activity.  Crucially, this does not lead
to degraded security guarantees as the client still understands the older
version and there is no expectation of enforcing security guarantees introduced
in newer versions.

### Clients Select Versions

In this strategy, the client determines the versions of metadata used in a
repository. For example, when creating a new attestation, the client may use the
latest version of the attestation predicate it supports. Similarly, when
creating new policy metadata (e.g., a new rule file), the client may use the
latest version of the policy metadata it supports. While this approach improves
usability as developers can use their existing clients without worrying about
version mismatches, it can lead to security gaps. For example, an older client
may inadvertently use a version of the policy metadata that has reduced security
guarantees. Similarly, this can lead to inconsistent policy states where
different policy metadata files have different versions, leading to potential
incompatibilities.

#### What if there are limits to which version a client can use?

Some of the concerns stated with letting clients select versions can be
mitigated by enforcing constraints on the versions of metadata that can be used.
But, this leads to added complexity. First, this needs to be implemented
correctly in the client. Second, the reason why certain versions are disallowed
may not be clear to the user, leading to confusion. Third, this still does not
prevent inconsistent policy states from being created, leading to potential
incompatibilities and security gaps. Finally, this approach complicates the
development of new policy schemas; the gittuf developers must also consider
permutations of compatibility across metadata versions, which is more
complicated than having to build translations between one policy version to the
next.

## Specification

Currently, gittuf versions the following pieces:
* attestation predicates: individual in-toto attestation predicates are
  versioned in the predicate type URI; this is inherited from the in-toto
  attestation specification
* gittuf implementation: the implementation itself is versioned and encompasses
  the behavior / workflows as well as the default versions of the metadata
  generated by the client at that version

The gittuf policy metadata's schema is not versioned, which will be amended as
part of this document. In summary, we intend to have the following gittuf
components versioned:
* attestation predicates
* gittuf policy metadata
* gittuf implementation

### Versioning Policy Metadata

All gittuf policy metadata schemas must henceforth have a `SchemaVersion` which
contains a URI indicating the schema version. For root metadata, this will be of
the form `https://gittuf.dev/policy/root/v<MAJOR>.<MINOR>.<PATCH>` and for rule
files, this will be of the form
`http://gittuf.dev/policy/rule-file/v<MAJOR>.<MINOR>.<PATCH>`. If the `PATCH` is
`0`, it is omitted. If the `MINOR` and `PATCH` are both `0`, they are omitted as
well.

The original gittuf policy metadata schema which is unversioned will be treated
as versions `https://gittuf.dev/policy/root/v0.1` and
`http://gittuf.dev/policy/rule-file/v0.1`.

While each policy metadata file will indicate its schema version, all gittuf
policy metadata in a policy state must have the same version to prevent
incompatibilities arising from mixing different versions of policy metadata. The
source of truth for the policy metadata versions is the version used for the
root metadata. This may be relaxed in a future update for patch version
differences.

When creating a new version of policy metadata, the gittuf developers must
implement how the previous version of the policy metadata schema is to be
translated to the new version. This enables the client to update existing
policies to their equivalents in the new version automatically.

#### Updating a Repository's Policy Metadata Version

When the root of trust key holders decide to upgrade the repository's policy
metadata to a newer major version, they must use a gittuf client that supports
the newer major version. The client must be used to update all policy metadata
files to the newer major version. To avoid requiring all policy metadata to be
re-signed by every required developer, the root key holders are trused to their
signature to all metadata.  Note that a threshold of root keys must be used to
sign all the updated policy metadata files, even if any of the files require
fewer signatures per the rules delegating to them.

#### Client Behavior

**Creating a new policy metadata file.** The client must create the new policy
metadata file with the version used for the latest root of trust metadata. If
the client supports newer versions of gittuf policy metadata (i.e., the
repository's metadata is of an older version), the client must print a warning
to the user indicating a newer version of gittuf policy metadata is available.
If the client does not support the policy metadata version used in the latest
root of trust metadata, the client must abort creating the new policy metadata
file.

**Loading policy metadata file.** The client presently verifies that the
metadata file is valid (i.e., signatures are valid, thresholds are met, etc.).
These checks are extended to include verifying that the policy metadata file's
version matches the version used in the latest root of trust metadata. If there
is a mismatch, the client must abort and indicate the version mismatch to the
user.

If the client supports newer versions of gittuf policy metadata than that used
in the latest root of trust, the client must print a warning to the user
indicating a newer version of gittuf policy metadata is available. If the client
does not support the policy metadata version used in the latest root of trust
metadata, the client must abort loading the policy metadata file and print a
warning to the user indicating the version mismatch. This is because the policy
metadata version may have specific security requirements that the client cannot
validate, degrading the security guarantees of gittuf.

**Modifying an existing policy metadata file.** As noted above, the client
verifies that the policy metadata file's version matches the version used in the
latest root of trust metadata when it loads the file. When there is no mismatch
and the client is able to load and validate the policy metadata file, the client
can proceed to modify the policy metadata file as directed by the user. When
writing the policy metadata file back to the repository, the client must ensure
that the same version is used as when it was loaded, even if the client supports
newer versions of the gittuf policy metadata.

Note that if the policy metadata file being modified is the root of trust
metadata itself, this does not apply. The root key holders may update the policy
version, meaning that the version will not match the version used in the latest
root of trust metadata. The client must allow this and update the versions of
all other policy metadata files in the repository to match the new version used
in the updated root of trust metadata.

**Signing an existing policy metadata file.** The client must verify that the
policy metadata file's version matches the version used in the latest root of
trust metadata when it loads the file. This is similar to the modification
workflow, meaning a client can be used to add a signature to an existing policy
metadata file as long as the client supports the version used in the latest root
of trust metadata. If the client does not support the policy metadata version
used, it must abort and print a warning to the user indicating the version
mismatch.

**Verifying gittuf policy.** The gittuf verification workflow also requires
loading policy metadata. So, the same version constraints apply. If the client
supports newer policy metadata versions than that used in the latest root of
trust, the client must print a warning to the user indicating a newer version of
gittuf policy metadata is available. Note that the client must not print
warnings for older policy metadata versions encountered in policy states prior
to the latest state. If the client does not support the version used in the
latest root of trust, then verification must be aborted as the client cannot
enforce policies it does not understand.  The client must continue enforcing
policies of older versions to ensure that a repository's historical changes
(which may have been protected by older versions) can continue being verified.

### Versioning Attestation Predicates

Attestation predicates are versioned via the predicate type URI as per the
in-toto attestation specification. The predicate type URI only indicates the
major version of the predicate for `v1` and above. Otherwise, the predicate type
URI indicates the major and minor version of the predicate.

#### Setting Predicate Versions for a Repository

The repository's root of trust metadata may indicate the minimum and maximum
versions supported for a predicate type. The minimum version prevents clients
from creating attestations with older versions of the predicate with reduced
security guarantees. The maximum version can be used to ensure compatibility
across clients used by developers contributing to the repository. Note that the
minimum version must be less than or equal to the maximum version.

#### Client Behavior

**Creating a new attestation.** The client must create the attestation using the
latest version of the predicate supported by the client that is within the
minimum and maximum versions specified in the repository's root of trust
metadata. If the root of trust only specifies a minimum version, the client must
use the latest version supported by the client as long as it is higher than the
specified minimum version. If the root of trust only specifies a maximum
version, the client must use either the latest version it supports or the
specified maximum version, whichever is lower. If the client does not support a
version that meets the repository's constraints, the client must abort creating
the attestation and print a warning to the user indicating the version mismatch.

```
selectedVersion = clientLatestVersion
if repoMinVersion is defined && selectedVersion < repoMinVersion:
    abort("Client does not support minimum predicate version")
if repoMaxVersion is defined && selectedVersion > repoMaxVersion:
    selectedVersion = repoMaxVersion
```

**Signing an existing attestation.** The client must only sign an existing
attestation when it supports the predicate version. If the client does not
support the predicate version (i.e., the client is outdated), it must abort and
print a warning to the user indicating the version mismatch. If the
attestation's predicate version does not meet the repository's minimum and
maximum version constraints, the client must also abort and print a warning to
the user indicating the version mismatch.

**Using attestations during verification.** The client must only use
attestations whose predicate versions meet the repository's minimum and maximum
version in the policy state applicable when the attestation was created. Any
attestations encountered that do not meet the version constraints must be
ignored.  Note that the client must continue supporting older predicate versions
to ensure gittuf verification can be used for historical repository activity.

### Versioning the gittuf Client

The gittuf implementation or client version conveys the behavior of gittuf
workflows it invokes. In that sense, this encompasses the version of the
predicate and policy metadata, as the client selects the version of metadata it
signs and consumes (subject to constraints indicated in the root of trust
metadata).

Significantly, the verification behavior of gittuf is only versioned via the
client's version, as this is not encoded as metadata itself. If the verification
behavior changes for existing policy and attestation predicate versions in a way
that a change that was previously allowed is now disallowed or vice versa, the
implementation's major version must be bumped. The gittuf client may check for
the existance of a newer version and inform the user. The client must not
attempt to update itself.

## Backwards Compatibility

This GAP does not have impact on backwards compatibility of the existing gittuf
design and implementation. Indeed, the GAP introduces granular versioning of
metadata and behavior, which impacts how gittuf handles backwards compatibility
going forward.

## Security

The GAP considers how policy metadata, attestation predicates, and the
implementation itself are versioned. In addition, the GAP specifies how clients
must behave when they encounter version mismatches. The GAP empowers root of
trust key holders to make decisions regarding the versions of metadata used in
the repository, both for policy and specific attestation predicates. More
generally, the GAP prevents rollback attacks for gittuf metadata (unless
explicitly allowed by the root key holders). Finally, the GAP ensures that
clients operating on metadata fail in scenarios where the client does not
understand some metadata version, ensuring that security guarantees are not
degraded.

## Prototype Implementation

Some of the changes proposed in this GAP such as policy metadata versioning have
been added to the gittuf implementation.
