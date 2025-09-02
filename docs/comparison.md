# Comparison: gittuf vs Guix vs sequoia-git

# Comparison: gittuf vs Guix vs sequoia-git

This document compares gittuf with other systems that integrate trust, signing, and verification into Git-based workflows.  
The criteria below are directly based on [gittuf’s stated goals](https://gittuf.dev/goals.html).

---

##  Comparison: Independent Policy Verification

| Feature | gittuf | Guix | sequoia-git |
|--------|:------:|:----:|:-----------:|
| Enables verification independent of SCP |  Policies & signatures stored in repo, verifiable by anyone with read access |  Package reproducibility can be independently verified |  Focuses mainly on commit signing; policy verification is limited |
| Uses cryptographic signatures for commits/policies |  Policies and commits signed with GPG/SSH/Sigstore keys |  Relies on reproducible builds; does not sign commits/policies directly |  Commit signing via OpenPGP |
| Designed to prevent bypass if SCP is compromised |  Metadata & policies in repo, SCP compromise doesn't break verification |  Less direct; relies on reproducibility |  Relies on SCP for policies; focuses mainly on signatures |

---

##  Comparison: Guardrails & Logs

| Feature | gittuf | Guix | sequoia-git |
|--------|:------:|:----:|:-----------:|
| Guardrails on who can update policies |  Policy can require multiple approvals & use delegations |  Maintainers control package definitions; no enforced multi-approval |  No built-in guardrails beyond commit signing |
| Append-only repository activity log |  Append-only log synchronized across clones |  Relies on standard Git history; no enforced append-only log |  Logs rely on SCP or external tools |
| Mitigates single point of failure |  Requires multiple maintainers for policy changes |  Maintainers still have direct control |  Single signer/maintainer can override |

---

##  Comparison: System Compatibility & Metadata

| Feature | gittuf | Guix | sequoia-git |
|--------|:------:|:----:|:-----------:|
| Works with existing Git repos |  Compatible with existing Git repos; only verifies activity after gittuf is initialized |  Separate from Git; manages packages & builds |  Extends Git via OpenPGP |
| Compatible with popular tools (GitHub, GitLab) |  SCP-agnostic; works alongside popular Git hosts |  Not built for standard Git workflows |  Works alongside existing platforms |
| Supports multiple signing mechanisms |  GPG, SSH, Sigstore |  GPG |  OpenPGP |

---
