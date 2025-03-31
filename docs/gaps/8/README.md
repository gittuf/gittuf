# Policy Inheritance across gittuf Repositories

## Metadata

* **Number:** 8
* **Title:** Policy Inheritance across gittuf Repositories
* **Implemented:** No
* **Withdrawn/Rejected:** No
* **Sponsors:** Aditya Sirish A Yelgundhalli (adityasaky), Patrick Zielinski (patzielinski), Dennis Roellke (dns43)
* **Related GAPs:** [GAP-4](/docs/gaps/4/README.md), [GAP-6](/docs/gaps/6/README.md), [GAP-7](/docs/gaps/7/README.md)
* **Last Modified:** March 25, 2025

## Abstract

gittuf is designed to enforce policies within a Git repository. The policy lives
within the repository, and is defined by the repository's maintainers. However,
this places significant policy management overhead on the maintainers of the
repository. Now, they must reason about what different policy configurations
mean, and whether some configuration meets security requirements/directives
_not_ specified as gittuf policy already (e.g., regulatory requirements, best
practices, standards). This GAP proposes **policy inheritance** as a solution.
Instead of the maintainers of every repository reasoning about and configuring
policies independently, they can choose to _inherit_ the policy controls. The
source of this inherited policy is some other gittuf-enabled repository. For
example, an open source foundation can define policies that all projects hosted
by the foundation inherit.

## Motivation

gittuf policies must currently be declared for each individual repository. This
adds policy management overhead in contexts with hundreds or thousands of
repositories. Also, this requires maintainers of each repository to individually
consider the gittuf rules they must declare to meet other requirements (e.g.,
organizational guidelines for code review).

Managing policies for each repository independently has other shortcomings. When
some requirement is updated, this must be reflected in every repository's gittuf
policy. Not only is this onerous but it can also lead to certain repositories
falling out of sync due to the overhead and logistics involved in updating all
of them.

## Specification

In gittuf, "policy" is a set of signed metadata stored in `refs/gittuf/policy`.
Thus, "inheriting" policy from another repository is implemented as storing
additional metadata in `refs/gittuf/policy` that matches the upstream
repository's policy metadata. Specifically, this is implemented using the gittuf
propagation pattern introduced in [GAP-7](/docs/gaps/7/README.md).

### Policy Inheritance via Propagation

The upstream repository whose policies are inherited is known as a "controller"
repository. A repository can choose to inherit policies from multiple controller
repositories. Each controller repository is declared in the inheriting
repository's root of trust metadata; when a controller repository is added, a
propagation directive is also added automatically to propagate the upstream
repository's `refs/gittuf/policy` reference into the inheriting repository's
`refs/gittuf/policy` reference.

A controller repository is identified using the following attributes:
* name: a unique (in the root of trust metadata) identifier for an upstream
  repository
* location: cloneable URL used to locate the canonical copy of the upstream
  repository
* initial root keys: the set of public keys used to verify the initial root of
  trust metadata of the upstream repository
* inherited attributes: list of policy semantics inherited from the specified
  upstream repository (e.g., [global rules](/docs/gaps/4/README.md), [code
  review tool attestations](/docs/gaps/6/README.md))

TODO: enumerate set of attributes that can be inherited

TODO: should this include initial root threshold?

### Allowing Policy Inheritance

A repository cannot inherit policies from any upstream gittuf-enabled
repository. The upstream repository must allow its policies to be inherited in
its root of trust metadata. Every upstream repository's policy state that is
inherited must indicate in its root of trust metadata that the policy can be
inherited.

If some upstream repository `U` that allowed inheriting its policy then disables
policy inheritance, the downstream repositories that have inherited its policy
must fail verification.

TODO: is "allowing inheritance" boolean or can the controller specify attributes
that can be inherited? Suggest boolean for simplicity

### Trusting Inherited Policy

The propagation workflow adds the upstream repository's policy metadata as a
subtree into the inheriting repository's policy reference. However, this policy
metadata is not automatically trusted. The initial root keys declared in the
controller definition is used to bootstrap trust for the upstream repository's
policy states. Here, gittuf's standard consecutive policy states verification
workflow is used to bootstrap trust for the specific policy version propagated
as a subtree.

### Enforcing Inherited Policy

As the upstream repository's policy metadata is inherited using the propagation
pattern, the full set of upstream policy metadata is included in the inheriting
repository. However, not all of the inherited metadata is used. Instead, only
the upstream repository's global rules, introduced in
[GAP-4](/docs/gaps/4/README.md), are enforced in the inheriting repository.
After the standard gittuf verification workflow, the global rules from each
inherited policy (i.e., each controller declared) are enforced against the
change in question.

## Reasoning

The gittuf design shows that policy can be stored and used like all other
content stored in the repository. [GAP-7](/docs/gaps/7/README.md) introduces the
propagation pattern that provides a content-agnostic gittuf-based mechanism for
managing updates to content tracked in different repositories. The observation
that policy management is a content tracking concern and the feature enabling
content tracking across the repository boundary can be combined to enable policy
metadata itself to live in one repository but apply across multiple
repositories.

Policy inheritance is technically possible without the propagation pattern.
During verification, a gittuf client could temporarily clone the controller
repository, load its policy, and use the specified attributes. However, the
propagation pattern ensures that policy updates in the upstream repository are
recorded in the downstream repository's RSL, along with a full copy of the
upstream repository's policy metadata. This provides protections against
metadata manipulation attacks by the upstream repository's maintainers.

## Backwards Compatibility

This GAP requires use of propagation. As such, if policy inheritance is
configured, then clients that interact with the repository need to be updated.

## Security

This GAP introduces some security concerns.

### Denial of Service due to Constraint Conflicts

The inherited policy may have a global rule that contradicts a global rule in
the downstream repository's policy (or from another controller's policy). In
such cases, verification will fail (as conflicting constraints can't all be
met), leading to denial of service. This GAP highlights the potential for this
issue but does not propose a solution. Instead, the maintainers of the
repository must decide out of band what to inherit from an upstream repository,
perhaps in consultation with the maintainers of the upstream repository.

### Controller Unavailability and / or Propagation Delays

A controller repository may not available, meaning a gittuf client operating on
an inheriting repository cannot check the controller for updates. If an update
is available, this would not be propagated over, and the downstream repository
will continue to use the previously propagated upstream policy. This must be
communicated to the downstream repository's user to allow them to decide whether
to continue using the previously propagated policy.

TODO: can downstream verifiers require verification to fail due to controller
unavailability?

### Impact of Controller Root Key Exposure

If the controller repository experiences a root key exposure below the required
threshold, its root of trust key holders can use the standard gittuf workflow to
rotate and revoke the exposed keys. The updated metadata will be propagated to
the inheriting repositories. The inheriting repositories do not need to update
the controller declaration as this does not impact the _initial set of root
keys_ for the upstream policy.

If a threshold of root keys are exposed, the upstream maintainers may choose to
reinitialize gittuf metadata. This would require the inheriting repository to
redeclare the controller with the new set of initial root keys.
