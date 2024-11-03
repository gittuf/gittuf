# Extending gittuf Policy to Multiple Grouped Repositories

## Metadata

* **Number:** 8
* **Title:** Extending gittuf Policy to Multiple Grouped Repositories
* **Implemented:** No
* **Withdrawn/Rejected:** No
* **Sponsors:** Aditya Sirish A Yelgundhalli (adityasaky), Patrick Zielinski (patzielinski), Dennis Roellke (dns43)
* **Related GAPs:** [GAP-4](/docs/gaps/4/README.md), [GAP-7](/docs/gaps/7/README.md)
* **Last Modified:** March 25, 2025

## Abstract

gittuf is designed to operate within the boundaries of a single Git repository.
This makes deploying gittuf across hundreds or thousands of repositories (e.g.,
in an enterprise context) complex as configuring policies across all
repositories can add significant management overhead. This GAP explores how
multiple repositories can share some gittuf metadata, allowing for some aspects
of repository policy to be declared in a single place but having it apply across
multiple repositories.

## Specification

To support multi-repository gittuf policies, this GAP proposes the following new
concepts:

* **repository network:** a repository network (or just network) is the
collective noun used to indicate a group of repositories that share some gittuf
metadata, even if the other contents of the repository are entirely distinct
* **controller repository:** a special repository that declares some gittuf
policy that is propagated to other repositories
* **network repository:** a repository that is part of a repository network
subject to directives from the controller repository

The controller repository's policy reference is configured so that policy
changes are propagated to a subdirectory in the policy reference of each
network repository. Propagation does not mean these policy contents are
actually used: the upstream policy's contents must be verified by the downstream
repository.

### Controller Repository Structure

The controller repository may be any Git repository that has had gittuf
initialized. Its root metadata indicates whether the repository serves a
controller for a network. If it serves as a controller for a repository network,
the root metadata also declares the repositories that are part of the network. A
controller repository can function as any other gittuf enabled repository with
standard rules and delegations that apply only to that repository.

TODO: add schema for root metadata with the controller field and identification
of network repositories

TODO: can a controller in turn belong to a network with another controller? in
other words, can this be nested?

### Network Repository Structure

A network repository belongs to one or more repository networks. Its root
metadata indicates the controller repository for each network the repository
belongs to.

TODO: does a network repo also include global rules? Do we need to handle
aggregation in the companion GAP about global constraints?

TODO: add schema for root metadata of network repository with one or more
controllers specified

### Repository Identification and Trust Bootstrapping

A controller repository must declare one or more repositories that are part of
its network. Similarly, a network repository must identify its controller
repositories. In both scenarios, each target repository (each network
repository for a controller repository, each controller repository for a
network repository) is achieved using the following pieces of information.

* Repository Location: A URL that can be used to locate the canonical copy of
the target repository.  The URL must include the scheme identifying the
transport protocol, and it must be possible to use the URL to clone or fetch
from the canonical copy of repository. For example, the canonical copy of the
gittuf repository is located at `https://github.com/gittuf/gittuf` and this URL
can be used to fetch the gittuf repository using standard Git tooling.
* Initial Root Keys: The public keys used to verify the signatures of the target
repository's initial root metadata, i.e., the very first _applied_ policy state.
These root keys need not be updated when keys themselves change as standard
gittuf semantics can be used to bootstrap trust for subsequent root metadata.
* Initial Root Threshold: The number of initial root keys that must have signed
the first applied root metadata in the target repository.

### Workflows

#### Create Network

A network is created by initializing a repository as the controller repository.

#### Add Repository to Network

First, the controller repository's metadata must be updated to declare the
network repository. Second, the network repository's RSL must be
configured to propagate changes from the controller repository's policy
reference. Also, the network repository's policy metadata must be updated to
join the network by identifying the controller repository using its location and
initial root keys. Once the network repository is configured to propagate
controller changes and has a way to trust the controller's policies (using the
configured root keys), the regular propagation workflow can be used to apply the
controller's policy into the network repository.

TODO: consecutive state verification, codify as workflow.

#### Remove Repository from Network

TODO: what does this handshake look like?

#### Verification of Changes in Network Repository

When invoking verification in a network repository, in addition to the
standard gittuf verification workflow, the verification workflow must apply the
global constraints for every controller policy that has been imported into the
network repository.

TODO: If the verification workflow is invoked from a controller repository for a
network repository, should it verify other controller policies it finds in
the network repos?

#### Verification of Propagation in Controller Repository

When a policy change is applied in a controller repository, the change will not
be propagated to all network repositories immediately. Instead, each
network repository will invoke the propagation workflow the next time a
correctly behaving gittuf client updates the repository's RSL. Thus, from the
perspective of the controller repository, there is a workflow that can be used
to check all the repositories part of the network to see if changes have been
propagated.

TODO: look for misbehaving clients? for eg., network repository's RSL is
updated from time 1 to time 2 but policy is not propagated. How / where is this
tracked by the controller?

## Motivation

Currently, gittuf must be deployed on a per-repository basis: each Git
repository has its own independent set of policy metadata. This makes it
difficult to scale gittuf when there are many repositories. We consider some
problems that arise with scaling gittuf to protect thousands of repositories, as
may be the case in enterprise contexts.

### Root Key Management

A repository's gittuf policy includes root of trust metadata which is signed by
the owners of the repository. The keys used to manage the root of trust must be
stored securely as all other policy metadata in the repository (primary rule
file, delegated rule files) ultimately derive their trust from the root keys.
Compromising a threshold of root keys for a repository would allow an attacker
to undermine the repository's gittuf policies.

When gittuf must be used at scale, across hundreds of thousands of repositories,
managing root keys on a per-repository basis is impractical. If every repository
must have dedicated root keys that are not used in any other repository, this
places significant overhead on repository owners to manage their keys securely,
especially when they may be the owners for multiple repositories. On the other
hand, if repositories share some root keys (when they also share some owners,
e.g., Alice uses the same key for the root role of all the repositories she
owns), the chances of exposure of a shared key is increased. The shared key must
be used every time any repository it controls must be updated, even if the
change made to each repository is identical (e.g., an enterprise-wide change in
policy that must be applied to each repository).

TODO: the current GAP does not actually address root key management, it's
focused on constraint propagation and verification...

### Enforcing Security Baselines

Organizations often want to set baseline security controls for multiple
repositories. For example, multiple repositories may be associated with a single
project and therefore have the same expectations with respect to the security
controls enforced. Managing these constraints independently for every repository
is onerous and can lead to certain repositories falling out of sync, due to the
cognitive overhead and logistics involved in updating all of them.

### Behavior / Workflow Goals

#### Controller Repository

In an organizational context, the owners of a network, i.e., the actors who
manage the controller repository, aim to set baseline security controls that
must be enforced against all network repositories. The controller's owners
must be able to validate that their baselines have been propagated to every
network repository. Additionally, the owners must be able to recursively
verify each network repository's state for one or more references, applying
both the repository's local constraints as well as the constraints declared by
the controller. If all repository's pass this validation, the entire network is
said to be in a valid state.

TODO: tie it to internal policy badging

#### Network Repository Developers

The developer of some network repository must be able to contribute to the
repository while abiding by both the local and controller's rules.

TODO: tie it to external policy badging (we inherit policy from baseline X)

## Reasoning

## Backwards Compatibility

This GAP has no impact on backwards compatibility.

## Security

### Constraint Aggregation Concerns

TODO: can we always aggregate global rules across a controller and network
repository? Possibly not, seems like it runs into issues for affected
namespaces.

### Controller / Network Unavailability

If a controller repository is unavailable, then verification in a network
repository must fail. Note that it is expected that the synchronization point
for the controller and network repositories are the same, so it is unlikely
only one is unavailable.

### Propagation Delay / Drift

A malfunctioning gittuf client may incorrectly not search the controller
repository for updates, meaning potential controller policy updates are not
propagated to the network repository. This is protected by the same "at least
one honest client" property of gittuf.

### Impact of Root Key Exposure

## References
