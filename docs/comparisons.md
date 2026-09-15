# Comparison with Other Systems

gittuf is not the only project working on strengthening trust in Git
repositories. This document compares gittuf's approach with two other
systems that address overlapping concerns: [GNU Guix's channel
authentication](https://guix.gnu.org/manual/en/html_node/Channel-Authentication.html)
and [sequoia-git](https://gitlab.com/sequoia-pgp/sequoia-git). It is meant to
help users understand the trade-offs and pick the tool that best matches
their threat model, not to declare one approach superior to another.

## GNU Guix Channel Authentication

Guix authenticates a channel's commit history using an
`.guix-authorizations` file, an S-expression-formatted file, at the root of
the repository. Every commit must be signed by an OpenPGP key listed in the
`.guix-authorizations` file of its parent commit(s); as maintainers join or
leave the project, they update this file to add or revoke keys. To bootstrap
trust for a new clone, users are given a "channel introduction": the first
commit at which the authorization invariant holds, along with the OpenPGP
key fingerprint that signed it. The `guix git authenticate` command (and
`guix pull` / `guix time-machine`) then verifies every commit back to that
introductory commit.

**Similarities with gittuf:** Both systems tie authorization to specific Git
commits, use a file committed to the repository itself to describe who can
make changes, and update that file's contents as trusted keys change over
time.

**Differences:** Guix's authorization file only expresses "who can commit
here," checked against the immediate parent's file; it does not natively
express per-path/per-branch policies, delegations, or a separate root of
trust independent from the authorization file's commit history. gittuf's
policy is expressed as signed [TUF](https://theupdateframework.io/)-style
metadata that supports fine-grained rules over specific Git references and
file paths, delegations of trust to other keys/teams, and a distinct root of
trust that is itself rotated and verified through its own metadata, rather
than being implicit in the "current HEAD" of an authorizations file. gittuf
also separates policy verification from the forge, recording a [Reference
State Log (RSL)](design-document.md#reference-state-log-rsl) to detect
force-pushes and out-of-band changes, which channel authentication alone
does not address.

## sequoia-git

sequoia-git is a specification and toolset (`sq-git`) for embedding a commit
signing policy directly in a Git repository, describing who is allowed to
add commits, cut releases, and modify the policy itself. Given a trust root
and a target commit, `sq-git log` verifies that there is a path from the
root to the target where every commit is authenticated according to the
policy in effect at that point, and it also checks for hard key revocations
that can retroactively invalidate commits signed with a revoked key.

**Similarities with gittuf:** Both express an explicit, versioned policy
that lives in the repository, verify a chain of commits against that
policy rather than trusting the forge, and support the policy itself being
updated (and re-verified) over time.

**Differences:** sequoia-git's policy and verification model centers on
OpenPGP keys and commit signing policy specifically, whereas gittuf is
signing-scheme agnostic (it supports GPG, Sigstore/keyless signing, and
other signing mechanisms via its TUF-based metadata) and extends its policy
model beyond commit authorization to also cover things like tag protection,
file/path-based rules, and propagation of policy across repositories via
[controller repositories](https://github.com/gittuf/gittuf/issues/880).
gittuf additionally maintains the RSL described above as an independent,
append-only record of repository activity to detect tampering that a purely
commit-signing-based policy cannot catch on its own (e.g., reference
deletion or force-pushes that rewrite history).

## Summary

| | gittuf | Guix channel authentication | sequoia-git |
|---|---|---|---|
| Policy format | TUF-style signed metadata | `.guix-authorizations` S-expression | Embedded signing policy file |
| Signing schemes | Signing-scheme agnostic (GPG, Sigstore, etc.) | OpenPGP | OpenPGP |
| Root of trust | Independent, rotatable root of trust metadata | Channel introduction commit + key fingerprint | Trust root commit |
| Scope | Refs, paths, tags, delegations, cross-repo policy propagation | Who may commit | Who may commit, release, and modify policy |
| Tamper detection beyond signing | Reference State Log (RSL) | Not addressed | Hard-revocation checks |
| Forge independence | Yes | Yes | Yes |

This comparison is not exhaustive; see [discussion
#1039](https://github.com/gittuf/gittuf/discussions/1039) for the
conversation that prompted it, and please open an issue or discussion if
something here is inaccurate or if there's another system worth comparing
against.
