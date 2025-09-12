# gittuf Roadmap

Last Modified: April 22, 2025

This document details gittuf's ongoing roadmap. As gittuf is under active
development, this document is not considered immutable, and some items may be
added or changed. The items are divided between those that are currently being
worked upon, those that are planned for the future, and those that we have
already completed.

## Partial / Work in Progress

### Support Developer Teams

gittuf currently identifies each developer by their signing key or identity.
Policies grant permissions to each individual developer. Eventually, gittuf must
support declaring teams of developers, with policies granting permissions to
those teams as a whole. Further, thresholds on required authorizations for
policies must be granular enough to apply across team boundaries. For example,
it must be possible to require two members of the development team and one
member of the security team to sign off on a change. This is not the same as a
total threshold of three across the members of the development and security
teams.

### Support For Different Hats (Roles)

Related to the concept of teams, is the concept that a single developer might be
on different teams and wish to choose how an action is perceived.  Suppose Alice
is both a maintainer and also on the security team.  She sometimes may be
approving something she authored (wearing her maintainer hat) and other times
doing a security review of a dependency (wearing her security hat).  It is
reasonable that she may want to control how a statement of trust by her is used.
These could naturally be linked to the teams for which a statement should be
trusted.

### Integrate with Git Ecosystem

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

This item is currently underway. The gittuf community has built a [GitHub
app](https://github.com/gittuf/github-app) that can be installed on a
repository. The app watches pull requests and records attestations for code
review approvals and pull request merges. The app also verifies gittuf policy
for the repository and adds a [status
check](https://docs.github.com/en/pull-requests/collaborating-with-pull-requests/collaborating-on-repositories-with-code-quality-features/about-status-checks)
to pull requests with the verification status.

### Read Permissions

gittuf's design implements _write_ permission policies such as who can write to
a Git reference or a file. This must be accompanied by support for _read_
permissions. This needs to be developed further as the feature can range from
the ability to store secrets all the way to maintaining encrypted objects for
certain Git references so only specific users can read that reference.

### Programmable Policy Extensions

gittuf implements write access control policies that pertain to whether changes
were made to a namespace in a Git repository (i.e. validating that a commit on
the main branch was created by an authorized person). However, other policies
may be important to users of a Git repository, such as mandating that all
commits have a DCO signoff. In these cases, gittuf should be able to allow
repository administrators to configure more open-ended policies.

[#765](https://github.com/gittuf/gittuf/pull/765) introduces the notion of a
_Programmable Policy Extension (PPE)_. This draws inspiration from
[Git hooks](https://git-scm.com/docs/githooks), but addresses many issues with
Git hooks today, such as secure distribution and sandboxing of these programs.

## Planned

### Develop Hash Algorithm Agility Extension

The
[hash algorithm agility](/docs/extensions/hash-algorithm-agility.md) extension
describes how gittuf can be used to maintain a record of object hashes using
stronger hash algorithms like SHA-256 while continuing to use SHA-1. While Git
is working on SHA-256 support, it is currently not backwards compatible with
existing repositories and unsupported by major Git hosts and forges. This
feature needs to be fleshed out as the current document merely records some
early ideas.

## Reached

### Reach Alpha Milestone

The gittuf implementation is built based on the specification defined in the
[design document](/docs/design-document.md). Therefore, as features are fleshed
out and built, the two are updated together.

Currently, gittuf is in a pre-alpha stage. Its core features are still being
developed, and therefore the primary item on the roadmap is building gittuf to
reach the alpha milestone. The gittuf alpha version must include support for the
main design document with features like policies for Git namespaces, file
namespaces, key distribution, the Reference State Log, and the ability to sync
gittuf metadata with remote repositories.

### Integrate in-toto Attestations

[in-toto](https://in-toto.io/) is a framework for comprehensive software supply
chain security. Of specific interest to gittuf is in-toto's Attestation
Framework that provides a standard way to express software supply chain claims.
By integrating support for source control specific in-toto attestations, gittuf
can also support verification against requirements specified by projects like
[SLSA](https://slsa.dev/).

As of April 2024, in-toto attestations can be used in gittuf for actions such as
approving a change in the repository. We are actively working upstream with the
SLSA project to enable using other predicate types that may be defined as part
of the source track.

### Dogfood gittuf

Once gittuf achieves sufficient maturity, the gittuf source must be protected
using gittuf. This will contribute significantly to the usability and further
development of the tool, and will demonstrate its features and viability.

### Reach Beta Milestone

After sufficient dogfooding and testing, gittuf should be released as a beta. As
a rule of thumb, post reaching beta, repositories should not have to
reinitialize gittuf due to breaking changes or incompatibility in metadata
formats.

### Support Multi-repository Policies

If gittuf operates only within the boundary of a single Git repository, scaling
gittuf across thousands of repositories becomes impractical. gittuf must
include mechanisms to enable reuse of gittuf policies across repositories so
that policy management overhead is minimized.