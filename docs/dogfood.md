# Dogfooding gittuf

Last Modified: April 24, 2024

As noted in gittuf's [roadmap](/docs/roadmap.md), we want to use gittuf to
secure the development of gittuf itself. Note that when we are dogfooding
gittuf, we do not expect the policy to remain consistent over time, especially
as gittuf itself may have breaking changes in the coming months. After gittuf
reaches v1, we expect to reset the policy and start over with a formal root
signing. We envision dogfooding to happen in several phases.

## Phase 1

At this stage, we will rely on automation to create and sign RSL entries on
behalf of the gittuf maintainers. While this is quite a bit less secure than
signatures issued directly by the maintainers, we believe this serves as a
starting point for us to feel gittuf's pain points ourselves. In addition to
signing RSL entries using sigstore online, we will be recording a GitHub
attestation of each pull request merged into the main branch. This will serve as
an auditable paper trail to inspect using gittuf in future.

## Phase 2

With command compatibility and improved usability of the gittuf tool, we will
begin transitioning to at least some RSL entries being issued by local keys held
by maintainers. This may also be accompanied by the development of helper tools
such as a gittuf merge bot that can verify whose signatures / approvals are
still needed in a pull request and present them with the commands to run to meet
those requirements.

## Phase 3

Finally, as gittuf nears v1, we expect to transition more seamlessly to
primarily offline signatures. This can, as before, only be achieved with further
usability improvements. In this final phase, we hope to essentially have worked
out the kinks with using gittuf actively so that we can proceed with a stable
release.
