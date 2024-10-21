# gittuf Augmentation Proposals (GAPs)

A gittuf Augmentation Proposal (GAP) is a design document that describes new
features, extensions, or changes to the gittuf design document. A GAP is a
**living document** that provides a concise technical specification of the
feature, describes the motivation for the change, and discusses the rationale
behind the design.

gittuf is an implementation-first project, unlike sister projects like in-toto
and TUF that are primarily specification projects. However, gittuf's
implementation includes a design document that describes how different aspects
of gittuf work (e.g., communication with forges, verification and recovery
workflows, etc.). The design document ensures gittuf's core features are
developed in a reasoned manner, which is especially important as gittuf is a
security project. GAPs ensure that changes to gittuf's design are similarly
developed in a reasoned manner with input from gittuf maintainers and community
members alike.

**Note:** A GAP cannot be used to propose changes to gittuf community processes,
structure, or governance. Process-related changes must instead be proposed to
the gittuf/community repository.

## List of GAPs

| Number | Title | Implemented | Withdrawn / Rejected |
|--------|-------|-------------|----------------------|
| 1 | [Providing SHA-256 Identifiers Alongside Existing SHA-1 Identifiers](/docs/gaps/1/README.md) | No | No |
| 2 | [gittuf on the Forge](/docs/gaps/2/README.md) | No | No |
| 3 | [Authentication Evidence Attestations](/docs/gaps/3/README.md) | No | No |
| 4 | [Supporting Global Constraints in gittuf](/docs/gaps/4/README.md) | No | No |
| 5 | [Principals, not Keys](/docs/gaps/5/README.md) | No | No |
| 6 | [Code Review Tool Attestations](/docs/gaps/6/README.md) | No | No |

## GAP Format

All GAPs must have the following sections to be merged into the gittuf
repository. A template of this document is available alongside this document.

1. **Metadata:** A list at the very top of a GAP that contains the GAP's number,
title, implemented status, withdrawn/rejected status, sponsors, contributors,
related GAPs and the date it was last modified.
1. **Abstract:** A short description of the GAP.
1. **Specification:** The technical specification of the proposed changes or
additions to the gittuf design.
1. **Motivation:** A description of the motivation for the proposed changes. The
motivation may precede the specification section in cases the context it
provides is important to reason about the specification.
1. **Reasoning:** A discussion of the reasoning behind specific design or
architectural decisions proposed in the specification. The sponsors should also
try and include summaries of discussions from the community related to these
decisions (perhaps raised on the pull request proposing the GAP or in
synchronous discussions in gittuf community meetings).
1. **Backwards Compatibility:** A discussion of how the proposed changes impact
the backwards compatibility of gittuf's design and implementation. If a GAP does
not break backwards compatibility, that must be stated explicitly.
1. **Security:** As gittuf is fundamentally a security project, any changes to
gittuf's design must be considered carefully with respect to how it changes the
security model of the project. Each GAP must discuss the security impact of the
proposed changes, potential issues that may arise as a consequence of the
proposed changes, their mitigations, and any implementation-specific footguns
developers must be mindful of.
1. **Changelog:** Every time a change to a GAP is **merged** into the
repository, an entry must be added to this section with a brief summary of the
changes and the date they were made. This section may be omitted for the very
first iteration of a GAP.
1. **References:** A list of references that include links to discussions
pertaining to the GAP as well as any external links relevant to the proposed
changes.

A GAP document may also include the following optional sections.

1. **Acknowledgements:** Any relevant acknowledgements to people (who are not
sponsors or contributors of the GAP) or projects (that in some way inspired the
GAP).

An **unimplemented** GAP may also include the following sections. These are
optional.

1. **Prototype Implementation:** A description or link to a prototype of the
proposed changes.

When a GAP is implemented, the document must be updated to reflect this in the
metadata section. Additionally, the following sections must be added to the
document.

1. **Implementation:** A description of how the GAP was implemented in gittuf.
If a prototype implementation was accepted as the final implementation, this
section may indicate as such and refer to the prototype implementation section.

If the features proposed in an implemented GAP are later removed, the GAP must
be updated to reflect this in the document's metadata section. The GAP's
reasoning must also be updated to indicate why the feature was removed.

## GAP Responsibilities

The participants in a GAP have specific responsibilities.

### Sponsor

Every GAP must have at least one sponsor. There is no limit to the number of
sponsors a GAP may have. Each sponsor must be listed in the GAP's metadata
section. The sponsors are the original proposers of a GAP and take on the
responsibility of authoring the document and submitting it for review by the
gittuf maintainers. Additionally, the sponsors should update the GAP based on
feedback or changes to the corresponding implementations, thus ensuring the
document reflects the latest status of the proposed changes.

### Contributor

A GAP may have one or more contributors. These are members of the community who
contribute to the GAP but do not wish to sponsor it. Each contributor must be
listed in the GAP's metadata section.

### gittuf Maintainers

Ultimately, the gittuf maintainers are responsible for overseeing GAPs and
keeping them updated. The responsibilities of the gittuf maintainers include
(but are not limited to):

* Engaging in discussions of problems to determine if a GAP is necessary
* Reviewing and providing feedback on a GAP in a timely manner with a focus on
  the impact of the proposed changes on gittuf's security model
* Ensuring the GAP follows the prescribed format
* Updating merged GAPs (if the sponsors do not) to reflect the state of their
  implementation; for example, if a GAP is implemented, if an implemented GAP
  feature is removed, or a GAP feature is updated significantly, the maintainers
  must ensure this is reflected in the living GAP document

Changes to GAP documents are subject to approval from the same threshold of
gittuf maintainers as all implementation changes. That is, if the implementation
requires two maintainers to approve some change, the same threshold applies to
changes to GAPs.

## GAP Workflow

A GAP may go through the following phases in its evolution from problem to
implemented solution.

### Discuss the problem

A GAP solves specific problems that the gittuf design currently does not. Rather
than directly approach the community with a GAP, it is a good idea to discuss
with the maintainers and the broader community the problem itself. This can help
confirm that the problem in fact exists (instead of being a misunderstanding of
the gittuf design), has not already been explored in a previous GAP, and a
solution ought to be part of gittuf (rather than another complementary system).
This discussion may happen in any forum used by the gittuf community, though the
repository's issue tracker is recommended for maximum visibility.

### Propose the GAP

After the maintainers agree that a GAP is required, one or more sponsors can
author a draft of the GAP. To submit a GAP, one of the sponsors must open a pull
request with the document to the gittuf repository. The GAP must follow the
format specified in this document (or copy the template provided alongside this
document), minus the number as that is assigned when the document is merged.

### Merging a proposed GAP

All proposed GAPs must be merged into the repository in a timely manner,
regardless of their implementation status. This increases the visibility of each
GAP, thus making it easier for other interested parties to discover the GAP,
propose further changes, and contribute to the implementation of the GAP.

The sponsors of a GAP may choose to withdraw their proposal or the maintainers
may choose to reject the proposed changes after assessing the GAP. Even in these
cases, the document must be merged into the repository with the status indicated
in the document's metadata section. Note that a GAP must only be withdrawn or
rejected on the basis of technical reasons (e.g., a better solution is proposed
or a security issue is discovered as a consequence of the proposal). The
reasoning section of the GAP must capture these technical considerations.

### Implementing a GAP

The changes proposed in a GAP may be implemented via patches to the gittuf
implementation. The changes to the implementation need not be submitted by the
sponsors of the GAP. When the gittuf maintainers think a GAP has been
implemented, they can propose an update to the document reflecting this. The
sponsors of a GAP may also propose marking a GAP as implemented, which is
subject to approval from the gittuf maintainers.

### Removing a GAP's implementation

After a GAP is implemented, the corresponding changes or feature additions may
be reverted (e.g., the feature leads to repeated security issues while being
used rarely or gittuf's design as a whole evolves in a way that makes the GAP
redundant). In such scenarios, the GAP must be updated to indicate this change.
In addition to the corresponding changes to the metadata section, other sections
such as the reasoning, implementation, backwards compatibility, and changelog
must also be updated.

## Acknowledgements

The GAP format and process in inspired by similar mechanisms in the in-toto
(in-toto Enhancements) and TUF (TUF Augmentation Proposals) communities.

## References

* [gittuf Design Document](/docs/design-document.md)
* [gittuf Maintainers](/MAINTAINERS.txt)
* [gittuf/community](https://github.com/gittuf/community)
* [GAP Template](/docs/gaps/template.md)
* [in-toto Enhancements (ITE)](https://github.com/in-toto/ite)
* [TUF Augmentation Proposals (TAPs)](https://github.com/theupdateframework/taps)
