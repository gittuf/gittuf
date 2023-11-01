# gittuf Roadmap

Last Modified: August 21, 2023

This document is a roadmap for the calendar year of 2023. As gittuf is under
active development, this document is not considered immutable, and some items
may be added or changed.

## Reach Alpha Milestone

The gittuf implementation is built based on the specification defined in the
[design document](/docs/design-document.md). Therefore, as features are fleshed
out and built, the two are updated together.

Currently, gittuf is in a pre-alpha stage. Its core features are still being
developed, and therefore the primary item on the roadmap is building gittuf to
reach the alpha milestone. The gittuf alpha version must include support for the
main design document with features like policies for Git namespaces, file
namespaces, key distribution, the Reference State Log, and the ability to sync
gittuf metadata with remote repositories.

## Integrate in-toto Attestations

[in-toto](https://in-toto.io/) is a framework for comprehensive software supply
chain security. Of specific interest to gittuf is in-toto's Attestation
Framework that provides a standard way to express software supply chain claims.
By integrating support for source control specific in-toto attestations, gittuf
can also support verification against requirements specified by projects like
[SLSA](https://slsa.dev/).

## Integrate with Git Ecosystem

Git forges like GitHub and GitLab allow repository owners to specify policies
such as the developers authorized to push to a branch, the developers who must
approve changes to certain files, and more. These repository policies can be
specified in gittuf, making conformance with repository policies publicly
verifiable. In addition, as gittuf tracks historic policies, auditing
repositories hosted on such forges at some older state can be made possible.

Another Git-specific tool that gittuf could integrate with is Gerrit, the code
review system. This integration, in combination with support for
[in-toto attestations](#integrate-in-toto-attestations) would allow for
transparent and auditable code review policy enforcement.

## Support Developer Teams

gittuf currently identifies each developer by their signing key or identity.
Policies grant permissions to each individual developer. Eventually, gittuf must
support declaring teams of developers, with policies granting permissions to
those teams as a whole. Further, thresholds on required authorizations for
policies must be granular enough to apply across team boundaries. For example, it
must be possible to require two members of the development team and one member
of the security team to sign off on a change. This is not the same as a total
threshold of three across the members of the development and security teams.

## Read Permissions

gittuf's design implements _write_ permission policies such as who can write to
a Git reference or a file. This must be accompanied by support for _read_
permissions. This needs to be developed further as the feature can range from
the ability to store secrets all the way to maintaining encrypted objects for
certain Git references so only specific users can read that reference.

## Develop Hash Algorithm Agility Extension

The
[hash algorithm agility](/docs/extensions/hash-algorithm-agility.md) extension
describes how gittuf can be used to maintain a record of object hashes using
stronger hash algorithms like SHA-256 while continuing to use SHA-1. While Git
is working on SHA-256 support, it is currently not backwards compatible with
existing repositories and unsupported by major Git hosts and forges. This
feature needs to be fleshed out as the current document merely records some
early ideas.

## Dogfood gittuf

Once gittuf achieves sufficient maturity, the gittuf source must be protected
using gittuf. This will contribute significantly to the usability and further
development of the tool, and will demonstrate its features and viability.
