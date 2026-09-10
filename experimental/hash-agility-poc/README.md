# gittuf GAP-1 (Hash Agility) Proof of Concept

**Authors:** Aarav, Aashish  
**Date:** September 2026  
**Context:** Empirical evaluation of Git SHA-1 -> SHA-256 migration strategies in gittuf for maintainers (Paulo Gomes, Patrick Zielinski).  
**Issue Tracking:** [gittuf/gittuf#104](https://github.com/gittuf/gittuf/issues/104) | [GAP-1 Specification](https://github.com/gittuf/gittuf/blob/main/docs/gaps/1/README.md)

---

## 1. Executive Summary

Git is actively transitioning from SHA-1 to SHA-256. Gittuf relies on signed metadata (the Reference State Log / RSL and TUF policies) containing embedded Git object hashes.

This Proof of Concept (PoC) evaluates three architectural solutions to hash agility:

```
                          ┌──────────────────────────┐
                          │   SHA-1 Git Repository   │
                          │   (gittuf initialized)   │
                          └─────────────┬────────────┘
                                        │
                         [ Migration to SHA-256 Repo ]
                                        │
           ┌────────────────────────────┼────────────────────────────┐
           │                            │                            │
           ▼                            ▼                            ▼
  [ Approach A ]               [ Approach B ]               [ Approach C ]
In-Memory Translation      Snapshot + Fresh Start       Cross-Signing Attestation
 (The Epoch System)         (Paulo's Route)              (The Provenance Bridge)
 ❌ REJECTED                ✅ PRIMARY RECOMMENDATION    ✨ OPTIONAL BRIDGE
 Signatures permanently     Clean epoch boundary, zero   in-toto DSSE statements
 break when translating     debt, anchored to Rekor      attest hash equivalence
```

---

## 2. Approach Analysis & Empirical Results

### Approach A: In-Memory Translation Layer (The Epoch System)
* **Design:** Retain SHA-1 metadata unchanged and dynamically translate hashes to SHA-256 in memory via Git's `compatObjectFormat`.
* **Empirical Result:** ❌ **REJECTED.**
* **Root Cause:** In Gittuf, RSL Reference Entries are **signed Git commit objects** where the `targetID: <sha1_hash>` is serialized directly into the commit message. Even if object lookups are mapped dynamically in-memory, **digital signature verification permanently fails** because the signed commit text contains the original SHA-1 hex string. Modifying the text invalidates the cryptographic signature.

### Approach B: Freeze + Snapshot + Fresh Start (Paulo's Path)
* **Design:** Treat migration as a hard epoch boundary.
  1. Freeze the SHA-1 repository.
  2. Compute a deterministic Merkle-style chain hash over all historical RSL entries.
  3. Export and sign a `SnapshotManifest` anchored to a transparency log (Sigstore / Rekor).
  4. Initialize fresh Gittuf metadata in the SHA-256 repository.
* **Empirical Result:** ✅ **RECOMMENDED AS PRIMARY STRATEGY.**
* **Strengths:** Cleanest codebase, zero runtime translation overhead, zero technical debt. The `pkg/gitinterface.Hash` type is already forward-compatible.

### Approach C: Cross-Signing in-toto Attestations (The Bridge)
* **Design:** Leverage Gittuf's native `internal/attestations` subsystem (`refs/gittuf/attestations`).
  1. Maintainers sign an in-toto DSSE statement declaring cryptographic equivalence between pre-migration SHA-1 commits and post-migration SHA-256 commits.
  2. The verifier queries this attestation when traversing historical commits.
* **Empirical Result:** ✨ **VIABLE & COMPLEMENTARY.**
* **Strengths:** Preserves historical signatures 100% intact while enabling unbroken provenance across the hash boundary.

---

## 3. Running the PoC Locally

To execute all three experimental paths and view the live findings report:

```bash
go run ./experimental/hash-agility-poc/
```

### Execution Output Summary
* **Phase 1:** Initializes dummy SHA-1 repo with 3 commits and logs 3 RSL entries.
* **Phase 2:** Converts to SHA-256 repo and captures native object resolution failure (`fatal: Not a valid object name`).
* **Phase 3:** Tests Approach A (translation) and documents signature breakdown.
* **Phase 4:** Tests Approach B (snapshot) and produces `snapshot-manifest.json`.
* **Phase 5:** Tests Approach C (attestations) and produces `hash-equivalence-attestation.json`.

---

## 4. Codebase Audit Findings

| File / Component | Status | Impact on Migration |
| :--- | :---: | :--- |
| `pkg/gitinterface/hash.go:57` | `NewHash()` accepts 40-char (SHA-1) and 64-char (SHA-256) | **Forward-compatible:** Core hash abstraction is already algorithm-agnostic. |
| `pkg/gitinterface/hash.go:52` | `ZeroHash` is hardcoded to 20 zero bytes (SHA-1) | **Action item:** Must be updated to be repository-format aware. |
| `internal/rsl/rsl.go` | RSL commit messages contain `targetID: <hash>` in signed text | **Key finding:** Confirms why translation layer cannot preserve signatures. |
| `pkg/gitinterface/commit.go:67` | `CommitUsingSpecificKey` uses `go-git` v5 (SHA-1 only) | **Action item:** Use Git CLI signing or upgrade to `go-git` v6 for SHA-256. |
| `internal/attestations/authorization.go:140` | `ReferenceAuthorizationPath()` embeds hash hex in tree paths | **Action item:** Must decouple path parser from fixed SHA-1 length. |

---

## 5. Artifact Schemas

### Snapshot Manifest (`snapshot-manifest.json`)
```json
{
  "schema_version": "gap1-poc-v1",
  "frozen_at": "2026-09-10T13:30:40Z",
  "sha1_repo_head": "e9afffcce72f4dad92289589f980d5840b4d100b",
  "rsl_tip": "d00b97620b6dfcea58d32bc18415d5aac41f9c13",
  "rsl_entries": 3,
  "rsl_chain_merkle_hash": "ae385e9bbfc57e9f3ca375de9e1fa0b4b85e3a67827c58f0c61a44ea3af7a9bb",
  "migration_note": "Repository frozen for SHA-1->SHA-256 migration. Old history verifiable via this manifest.",
  "signed_by": "maintainer-key"
}
```

### In-Toto Hash Equivalence Attestation (`hash-equivalence-attestation.json`)
```json
{
  "payloadType": "application/vnd.in-toto+json",
  "payload": "eyJfdHlwZSI6ICJodHRwczovL2luLXRvdG8uaW8vU3RhdGVtZW50L3YxIiwgInByZWRpY2F0ZVR5cGUiOiAiaHR0cHM6Ly9naXR0dWYuZGV2L3ByZWRpY2F0ZS9oYXNoLWVxdWl2YWxlbmNlL3YxIn0...",
  "signatures": [
    {
      "keyid": "root-maintainer-key-1",
      "sig": "5a7f920bc8b603..."
    }
  ]
}
```

---

## 6. Recommended Action Plan for GAP-1

1. **Adopt Approach B as the canonical migration standard** for `gittuf migrate sha256`.
2. **Support Approach C attestations** for enterprise repositories requiring cryptographic bridge verification across archives.
3. Fix `ZeroHash` in `pkg/gitinterface/hash.go` to dynamically respect `core.repositoryformatversion`.