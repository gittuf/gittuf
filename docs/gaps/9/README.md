# Enforcing Consistency in Policy Inheritance

## Metadata

* **Number:** 9
* **Title:** Enforcing Consistency in Policy Inheritance
* **Implemented:** No
* **Withdrawn/Rejected:** No
* **Sponsors:** Aditya Sirish A Yelgundhalli (adityasaky), Patrick Zielinski (patzielinski), Dennis Roellke (dns43)
* **Related GAPs:** [GAP-4](/docs/gaps/4/README.md), [GAP-7](/docs/gaps/7/README.md), [GAP-8](/docs/gaps/8/README.md)
* **Last Modified:** March 25, 2025

## Abstract

This GAP extends the notion of policy inheritance introduced in
[GAP-8](/docs/gaps/8/README.md). While a repository may choose to inherit policy
from some upstream gittuf-enabled repository, the policy inheritance feature
does not enable the upstream repository to verify that some declared set of
repositories have inherited its policies.

## Motivation

By default, gittuf policies must be enabled on a per-repository basis. To
address the overhead introduced by this, [GAP-8](/docs/gaps/8/README.md) made it
possible for a repository to inherit policies from some other upstream
"controller" repository. This allows for gittuf to be used across hundreds or
thousands of repositories. However, policy inheritance does not answer the
question of whether a specific repository chose to inherit the policy.
Organizations frequently have requirements such as ensuring all the repositories
related to some software product meet minimum security baselines. These
baselines can be declared in a controller repository and inherited by all the
repositories where the baselines must be applied. But, there must also be a way
to _verify_ that all the relevant repositories have in fact inherited the
policies as expected.

## Specification

Note: several concepts such as a "controller repository" are taken directly from
[GAP-8](/docs/gaps/8/README.md).

A repository that chooses to act as a "controller", i.e., its policies can be
inherited by other repositories, can also declare specific repositories that
must inherit its policies. Each of these repositories, known as a "network
repository", is identified using the same attributes used by a repository to
declare a controller repository (see [GAP-8](/docs/gaps/8/README.md)).

This allows the controller repository to _enforce_ the set of repositories that
_must_ inherit its policies. A gittuf client operating over the controller
repository can verify that each of the network repositories have declared the
controller in their root of trust metadata. Further, the client can also
identify network repositories that have not propagated the latest controller
policy.

TODO: is this attested to upstream? witness entry in the controller's RSL?

TODO: does verification only check for policy propagation or does it also verify
the network repository's RSL against policy? Full verification is complicated,
we don't know the parameters, potentially...

<!---### Workflows

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

TODO: tie it to external policy badging (we inherit policy from baseline X) -->

## Reasoning

TODO

## Backwards Compatibility

This GAP has no impact on backwards compatibility.

## Security

TODO

## References

TODO
