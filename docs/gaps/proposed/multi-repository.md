# Extending gittuf Policy to Multiple Grouped Repositories

## Metadata

* **Number:**
* **Title:** Extending gittuf Policy to Multiple Grouped Repositories
* **Implemented:** No
* **Withdrawn/Rejected:** No
* **Sponsors:** Aditya Sirish A Yelgundhalli (adityasaky), Patrick Zielinski (patzielinski), Dennis Roellke (dns43)
* **Related GAPs:** [GAP-4](/docs/gaps/4/README.md)
* **Last Modified:** January 22, 2025

## Abstract

gittuf is designed to operate within the boundaries of a single Git repository.
This makes deploying gittuf across hundreds or thousands of repositories
impractical. This GAP explores how multiple repositories can share some gittuf
metadata while still allowing each individual repository sufficient flexibility
in declaring its rules.

## Specification

This GAP proposes the "propagation" pattern for gittuf repositories.
Taken generally, the propagation pattern defines the mechanism to take some
changes in an upstream gittuf-enabled repository and make it available in
another, downstream gittuf-enabled repository. Additionally, the pattern
introduces some changes to standard gittuf workflows to ensure that subsequent
changes are also propagated, and there is a record of the states clients see of
upstream or downstream repositories.

Next, this GAP describes how this pattern can be used to propagate gittuf policy
across multiple repositories. This introduces changes to gittuf workflows as
well, related to how a gittuf client uses the propagated policy during
verification.

### gittuf Propagation

gittuf propagation is a mechanism to check in the contents of an upstream
gittuf-enabled repository at a specified "tracked" reference into a path in one
or more references in a downstream gittuf-enabled repository. Propagation
applies changes into the downstream repository when the upstream repository's
RSL indicates there's an update to the tracked reference. The contents at the
revision indicated by the upstream repository's RSL entry for the reference are
copied into the specified path in the references in the downstream repository.

#### Recording Propagation in RSL

For each reference, the propagation workflow records a new entry in the RSL
indicating that the changes were propagated over.

The propagation entry has the following structure:

```
RSL Propagation Entry

ref: <local ref name>
targetID: <local target ID>
upstreamRepository: <upstream repository location>
upstreamEntryID: <upstream RSL reference entry ID>
number: <number>
```

The propagation entry has all the fields as a regular reference entry. It also
identifies the entry of the upstream repository that was propagated over.

#### Handling Revoked Propagated Changes

If the upstream entry is revoked after being propagated to a downstream
repository, the next propagation check in the downstream repository identifies
that the latest unrevoked upstream entry is different, and thus the revoked
changes will be replaced. In other words, a special revocation flow is not
necessary for propagation.

#### Execution of Propagation Workflow

When a downstream repository is updated, i.e., a new RSL entry is created, the
gittuf client must also check the upstream repository's RSL for any updates. If
the upstream RSL has changes that must be propagated over, the propagation must
be executed before the downstream repository's RSL is updated with any other
changes. This ensures that changes are propagated as quickly as possible to
downstream repositories, and are applied to the RSL prior to any other
downstream changes.

The propagation workflow is as follows:

1. For each upstream repository `U` configured for propagation in downstream
   repository `D`:
    1. Temporarily clone and fetch `U` along with its RSL
    1. For each reference `R` tracked in `U` for propagation:
        1. Identify the last propagation entry for `R` in `D`'s RSL
        1. Identify latest unskipped entry for `R` in `U`'s RSL
        1. If the latest unskipped entry is different from that recorded in last
           propagation entry:
            1. Fetch `R` to the temporary clone of `U`
            1. Copy tree of `R`'s commit to specified path in `D`
            1. Record propagation entry in `D`

TODO: is this significant overhead / propagation for every push if the upstream
repository has significant traffic?

TODO: do we want to add "witness" entries as well? A gittuf client says I
checked upstream and saw RSL entry X, nothing to propagate. Next time, a client
needn't check beyond entry X. However, we must be careful with how this is
verified as a malicious client could skip propagating and then say nothing to
propagate.

TODO: what if we propagate changes from upstream to `foo` but also have a
`file:foo/*` policy in place? Like the recovery flow, can anyone propagate?
should propagation be the responsibility of only authorized users if the
directory is protected? I lean towards the former.

TODO: what if propagation into `foo/*` happens at upstream revision A, then
downstream modifies contents, then upstream has revision B? Should propagation
recognize that local changes have been made and abort? Should propagation
overwrite changes, i.e., we make it as clear as possible that this pattern must
not be used if "automatic" propagations will overwrite changes?

#### Configuring Upstream Repositories for Propagation

TODO: where should propagation be configured? It's tempting to make this purely
an RSL function so propagation works well without policies.

### Propagating gittuf Policy

To support multi-repository gittuf policies, this GAP proposes adding the
following concepts:

* **repository network:** a repository network (or just network) is the
collective noun used to indicate a group of repositories that share some gittuf
metadata
* **controller repository:** a special repository that declares some gittuf
policy that is propagated to other repositories
* **controlled repository:** a repository that is part of a repository network
subject to directives from the controller repository

The controller repository's policy reference is configured so that policy
changes are propagated to a subdirectory in the policy reference of each
controlled repository. Propagation does not mean these policy contents are
actually used: the upstream policy's contents must be verified by the downstream
repository.

TODO: the configuration to propagate the upstream policy ref depends on what we
decide for configuring propagation generally in a downstream repository.

#### Controller Repository Structure

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

#### Controlled Repository Structure

A controlled repository belongs to one or more repository networks. Its root
metadata indicates the controller repository for each network the controlled
repository belongs to.

TODO: does a controlled repo also include global rules? Do we need to handle
aggregation in the companion GAP about global constraints?

TODO: add schema for root metadata of controlled repository with one or more
controllers specified

#### Repository Identification and Trust Bootstrapping

A controller repository must declare one or more repositories that are part of
its network. Similarly, a controlled repository must identify its controller
repositories. In both scenarios, each target repository (each controlled
repository for a controller repository, each controller repository for a
controlled repository) is achieved using the following pieces of information.

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
controlled repository. Second, the controlled repository's RSL must be
configured to propagate changes from the controller repository's policy
reference. Also, the controlled repository's policy metadata must be updated to
join the network by identifying the controller repository using its location and
initial root keys. Once the controlled repository is configured to propagate
controller changes and has a way to trust the controller's policies (using the
configured root keys), the regular propagation workflow can be used to apply the
controller's policy into the controlled repository.

TODO: consecutive state verification, codify as workflow.

#### Remove Repository from Network

TODO: what does this handshake look like?

#### Verification of Changes in Controlled Repository

When invoking verification in a controlled repository, in addition to the
standard gittuf verification workflow, the verification workflow must apply the
global constraints for every controller policy that has been imported into the
controlled repository.

TODO: If the verification workflow is invoked from a controller repository for a
controlled repository, should it verify other controller policies it finds in
the controlled repos?

#### Verification of Propagation in Controller Repository

When a policy change is applied in a controller repository, the change will not
be propagated to all controlled repositories immediately. Instead, each
controlled repository will invoke the propagation workflow the next time a
correctly behaving gittuf client updates the repository's RSL. Thus, from the
perspective of the controller repository, there is a workflow that can be used
to check all the repositories part of the network to see if changes have been
propagated.

TODO: look for misbehaving clients? for eg., controlled repository's RSL is
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
must be enforced against all controlled repositories. The controller's owners
must be able to validate that their baselines have been propagated to every
controlled repository. Additionally, the owners must be able to recursively
verify each controlled repository's state for one or more references, applying
both the repository's local constraints as well as the constraints declared by
the controller. If all repository's pass this validation, the entire network is
said to be in a valid state.

#### Controlled Repository Developers

The developer of some controlled repository must be able to contribute to the
repository while abiding by both the local and controller's rules.

## Reasoning

### Propagation vs Git Submodules

TODO

### Propagation vs Git Subtree

TODO

## Backwards Compatibility

TODO

## Security

### Constraint Aggregation Concerns

TODO: can we always aggregate global rules across a controller and controlled
repository? Possibly not, seems like it runs into issues for affected
namespaces.

### Controller / Controlled Unavailability

If a controller repository is unavailable, then verification in a controlled
repository must fail. Note that it is expected that the synchronization point
for the controller and controlled repositories are the same, so it is unlikely
only one is unavailable.

### Propagation Delay / Drift

A malfunctioning gittuf client may incorrectly not search the controller
repository for updates, meaning potential controller policy updates are not
propagated to the controlled repository. This is protected by the same "at least
one honest client" property of gittuf.

### Impact of Root Key Exposure

## References
