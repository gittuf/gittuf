# Supporting Global Constraints in gittuf

## Metadata

* **Number:** 4
* **Title:** Supporting Global Constraints in gittuf
* **Implemented:** No
* **Withdrawn/Rejected:** No
* **Sponsors:** Aditya Sirish A Yelgundhalli (adityasaky), Patrick Zielinski (patzielinski)
* **Last Modified:** January 21, 2025

## Abstract

gittuf implements the "delegation" mechanism for specifying the actors trusted
to make changes to repository namespaces. These mechanisms cannot be used by
repository owners to set baseline security controls or controls that are not
typical delegations of trust. This GAP introduces "global constraints", declared
in a repository's root of trust, to address this shortcoming.

## Specification

This GAP introduces the notion of "global constraints" (also referred to as
"global rules").

### Declaring Global Constraints

Global constraints are declared by the repository's owners in the root of trust
metadata. Unlike the delegations implemented in gittuf today, global constraints
can be of distinct types, with each type indicating how it is verified by a
gittuf client. As with any changes to root of trust metadata, adding, removing,
or updating a global constraint requires approval from a threshold of repository
owners.

### Verifying Global Constraints

The gittuf verification workflow is extended to include the verification of all
applicable global constraints after the standard workflow protecting the
namespace being verified. The workflow used to verify a specific constraint
depends on its type, and is discussed below alongside the proposed constraint
types.

### Global Constraint Schema

Each global rule MUST have a name. This name MUST be unique among other global
rules (irrespective of type), though the implementation may determine if the
name must also be unique among all delegation names as well.

Depending on the type of the global rule, additional information may be
necessary. The extension defers the specific schema of global rules to the
implementation, as different policy metadata versions may require different
schemas.

### Global Constraint Types

This extension builds on the motivating examples and proposes two types of
constraints. In time, more types may be added and these MUST be recorded in this
extension document.

#### Minimum Threshold

Another common security control is the ability to require a minimum threshold of
approvals for changes to some namespace, without also specifying the specific
actors trusted for these approvals. Thus, the `threshold` global constraint
requires one or more patterns that identify the namespaces protected by the
constraint as well as a numeric threshold value.

The `threshold` constraint is verified against the set of accepted actors
identified by the standard verification workflow. Consider a delegation that
protects the `main` branch requiring two of Alice, Bob, and Carol to approve
changes. The successful termination of the standard verification workflow
requires two signatures to be verified, and the workflow returns the
corresponding two actors. The global constraint is then verified against this
returned set to see if the number of verified actors meets the global
constraint's threshold.

Depending on the existence and configuration of delegations protecting the same
namespace as a global constraint, several situations are possible.

##### Delegation(s) exist and they all require a threshold equal to or higher than the global constraint

In this scenario, there may be multiple delegations (at multiple levels) all
protecting the same namespace as a threshold global constraint. If all
delegations have a threshold equal to or higher than that declared in the global
constraint, then the global constraint will always be satisfied. This is similar
to the scenario described above.

##### Delegation(s) exist and only some require a threshold equal to or higher than the global constraint

There may be multiple delegations (at multiple levels) of which only some
require a threshold equal to of greater than that of the global constraint.
Consider for example that the primary rule file requires two of Alice, Bob, and
Carol to approve changes to the namespace. In turn, a threshold of Alice, Bob,
and Carol delegate trust for this namespace to Dana and Erin, but require only a
threshold of one.

In this scenario, when the standard verification workflow successfully verifies
signatures against the Alice, Bob, and Carol delegation (i.e., two of the three
have approved the changes), then the global constraint is met as well. However,
if only one of Dana and Erin issue a signature, while this satisfies the
standard verification workflow, this does not satisfy the global constraint.

TODO: what if both Dana and Erin sign despite only one needing to? Should we
verify exhaustively and return both, thus meeting the global constraint?

##### Delegations do not exist but a threshold global constraint is set

When no delegations exist protecting a namespace, the standard verification
workflow terminates without verifying any signatures. This extension proposes a
special case for this scenario where no delegations exist but a global
constraint does. Specifically, all actors' keys declared across all metadata are
used to verify the signatures associated with the change (RSL entry signature,
reference authorizations, other attestations as applicable). The assumption here
is that any actor declared in the metadata is trusted to make write changes to a
namespace that is not protected by any explicit delegations. This may be updated
in a future version of gittuf policy metadata that allows declaring granular
permissions on a per-actor basis.

#### Prevent Force Pushes

A force push results in the history of the branch being rewritten. Thus, the
`prevent-force-pushes` global constraint prevents rewriting history for the
specific Git references. As such, this contraint requires one or more patterns
that identify the repository references protected by the constraint.

This constraint type is verified for every entry for a reference protected by
the constraint. When verifying an RSL reference entry, the previous unskipped
RSL reference entry for the same reference is identified. To meet this
constraint, the verification workflow determines if the current RSL reference
entry's target commit is a descendent of the previous RSL reference entry's
target commit. If yes, then the constraint is met. If not, then verification
terminates with an error indicating the rule that was not met.

### Example

The following snippet shows the declaration of both types of rules in a
repository's root of trust metadata.

TODO: add example snippet and explanation.

## Motivation

gittuf currently supports explicitly granting write permission to a namespace to
a specific set of actors as well as a threshold that identifies the number of
actors that must agree to a change to that namespace. This extension of trust
uses the concept of "delegation", and a set of actors granted some trust can
choose to delegate this further to other actors.

This mechanism does not support setting generic controls over changes in the
repository, such as to enforce a baseline security posture _irrespective of the
configuration of specific delegations controlling who can make changes to a
namespace_. This GAP considers some motivating scenarios that influence the
design of global constraints. These are not exhaustive and may be expanded at a
later date.

### Minimum Threshold

Organizations frequently look to require a minimum number of approvers for
source code changes, irrespective of the specific actors who are trusted as
approvers for some repository namespace, which can be declared using standard
rules / delegations. For example, a large monorepo may include several levels of
delegations determining which actors are trusted for which subprojects. The
repository owners wish to set a minimum threshold of two for all changes to a
namespace irrespective of the specific subprojects or the actors trusted,
leaving that to the more specific delegations. To achieve this in gittuf without
global constraints, the owners must ensure every delegation that matches the
namespace has a threshold of at least two, which is impractical.

### Prevent Force Pushes

A repository's owners may choose (perhaps to conform with the upcoming SLSA
source track that requires this constraint) to block force pushes to some
branches, thus preventing alterations of their histories. This ensures
continuity of important repository branches meant for downstream consumption.
There is currently no way to enforce such a rule using gittuf's delegations;
indeed, this constraint isn't a delegation of trust at all, but rather a
specific property that must be enforced for repository changes.

### Specify Merge Strategies

A repository's owners may choose to enforce specific merge strategies (e.g.,
always create a merge commit, merge using a squashed commit that applies a
change atomically, etc.).

### Require Execution of Additional Checks

A repository's owners may require additional source code checks to be executed
prior to some change being merged. For example, a repository may require its
unit tests to be executed for every change, and expect all checks to pass for
the change to be merged. Another example is the execution of linters and secrets
scanners that enforce source code quality and hygiene.

## Reasoning

The GAP introduces the generic notion of "global constraints" that can be
extended to support a variety of repository-wide security controls.

### Limiting to Root of Trust

Global constraints must be declared in a repository's root of trust as they are
repository-wide constraints, and not specific to any one rule file. In the
future, there may be a preference to move (or otherwise support) this in the
primary rule file, for repositories where there is a strict separation between
what the root of trust and primary rule file metadata are used for. This is
currently not part of the GAP so as to keep rule file schemas consistent across
primary and delegated rule files.

### Supporting Types of Global Constraints

This GAP introduces the notion of global constraints and does not further group
specific constraint types together based on any shared characteristics. This is
because, ultimately, each security control that is supported via a global
constraint type is likely to have a unique verification workflow that must be
added to the implementation. Over time, the implementation (and this GAP) may
evolve in a way where different constraint types that share verification
characteristics are grouped together.

### Alternative Solution: User Programmable Checks

Supporting types of global constraints in gittuf may not be preferable due to
maintainability concerns. Additionally, changes in semantics of a named
constraint type can cause inconsistencies in verification. An alternative
approach may be to introduce a generic programmable layer into gittuf. In such a
model, global constraints would be expressed as small check programs executed in
a pre-determined environment built into gittuf.

TODO: connect this with gittuf + lua / hooks work

## Backwards Compatibility

This GAP impacts backwards compatibility in certain cases. If a repository's
metadata does not declare global constraints, any version of gittuf (with or
without support for global constraints) can be used for verification. If a
repository's metadata declares global constraints, then a version of gittuf
released prior to the addition of this feature will ignore the global rules
altogether. Additionally, even a gittuf client with knowledge of the concept of
global constraints may not support a specific type of constraint. In such
scenarios, the client is unable to verify the unsupported global constraint(s),
and must abort verification with a message to the user to update their gittuf
client.

## Security

Adding more mechanisms or types of rules to gittuf does not inherently pose a
security threat. The concerns with security relate to incompatibility of
clients, similar to those discussed in the backwards compatibility section.

Essentially, support for global constraints must be added in a manner such that
an older client can:
* abort verification with a warning when it encounters an unrecognized global
constraint type
* preserve global constraints when making changes to the repository's root of
trust metadata, even if the client entirely lacks support for global constraints

## Prototype Implementation

Initial support for global constraints has been implemented as a gittuf
developer mode feature.

## References

* [SLSA Source Track Draft](https://slsa.dev/spec/draft/source-requirements)
