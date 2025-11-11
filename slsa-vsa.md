# gittuf and the SLSA Source Track

gittuf may be used to generate SLSA Source Track VSAs and source provenance.
This document assumes that you have some knowledge of [SLSA](https://slsa.dev)
and the [SLSA Source Track](https://slsa.dev/spec/draft/source-requirements).

You can generate SLSA Source Track attestations by way of verifying a
gittuf-enabled repository with the `verify-ref` command. The following is a demo
of how to generate these attestations. Note that we assume you already have a
gittuf-enabled repository to generate these attestations from

## Source VSAs

[Source VSAs](https://slsa.dev/spec/draft/source-requirements#source-verification-summary-attestation)
offer a high-level snapshot of what SLSA Source Level a repository is compliant
to. In gittuf, we may generate and export such attestations by using the
`write-source-verification-summary` flag, like so:

```bash
gittuf verify-ref --write-source-verification-summary vsa.bundle main
```

This would write a Source VSA named `vsa.bundle` upon verifying the `main`
branch in the repository.

## Source Provenance Attestations

[Source Provenance Attestations](https://slsa.dev/spec/draft/source-requirements#source-provenance-attestations)
offer a more detailed view into how a certain SLSA Source Level may be claimed.
gittuf is able to generate and export such attestations via the
`--write-source-provenance-attestations` flag, like so:

```bash
gittuf verify-ref --write-source-provenance-attestations spa.bundle main
```

This would write a source provenance attestation bundle nameed `spa.bundle`
upon verifying the `main` branch in the repository.
