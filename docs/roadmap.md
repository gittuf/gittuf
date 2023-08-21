# gittuf Roadmap

Last Modified: August 21, 2023

This document is a roadmap for the calendar year of 2023. As gittuf is under
active development, this document is not considered immutable, and some items
may be added or changed.

## Reach Alpha Milestone

While gittuf is primarily an implementation, its design is documented in the
[specification](/docs/specification.md). Therefore, as features are fleshed out
and built, the two are updated together.

Currently, gittuf is in a pre-alpha stage. Its core features are still being
developed, and therefore the primary item on the roadmap is building gittuf to
reach the alpha milestone. The gittuf alpha version must include support for the
main specification document with features like policies for Git namespaces, file
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

## Read Permissions

gittuf's specification implements _write_ permission policies such as who can
write to a Git reference or a file. This must be accompanied by support for
_read_ permissions. This needs to be developed further as the feature can range
from the ability to store secrets all the way to maintaining encrypted objects
for certain Git references so only specific users can read that reference.

## Develop Hash Algorithm Agility Extension

The
[hash algorithm agility](/docs/extensions/hash-algorithm-agility.md) extension
describes how gittuf can be used to maintain a record of object hashes using
stronger hash algorithms like SHA-256 while continuing to use SHA-1. While Git
is working on SHA-256 support, it is currently not backwards compatible with
existing repositories and unsupported by major Git hosts and forges. This
feature needs to be fleshed out as the current document merely records some
early ideas.
