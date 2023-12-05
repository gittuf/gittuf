# gittuf

[![OpenSSF Best Practices](https://www.bestpractices.dev/projects/7789/badge)](https://www.bestpractices.dev/projects/7789)

gittuf provides a security layer for Git using some concepts introduced by [The
Update Framework (TUF)]. Among other features, gittuf handles key management for
all developers on the repository, allows you to set permissions for repository
branches, tags, files, etc., lets you use new cryptographic algorithms (SHA256,
etc.), protects against [other attacks] Git is vulnerable to, and more — all
while being backwards compatible with GitHub, GitLab, etc.

gittuf is a sandbox project at the [Open Source Security Foundation (OpenSSF)]
as part of the [Supply Chain Integrity Working Group].

## Current Status

gittuf is currently in alpha. It is NOT intended for use in a production system
or repository. Contributions are welcome, please refer to the [contributing
guide]. Some of the features listed above are being actively developed, please
refer to the [roadmap] and the issue tracker for more details.

## Installation

This repository provides pre-built binaries that are signed and published
using [GoReleaser]. The signature for these binaries are generated using
[Sigstore], using the release workflow's identity. Make sure you have
[cosign] installed on your system, then you will be able to securely download
and verify the gittuf release:

```sh
ARCH=amd64
OS=linux
VERSION=0.1.0
cd $(mktemp -d)

curl -LO https://github.com/gittuf/gittuf/releases/download/v${VERSION}/gittuf_${VERSION}_${OS}_${ARCH}
curl -LO https://github.com/gittuf/gittuf/releases/download/v${VERSION}/gittuf_${VERSION}_${OS}_${ARCH}.sig
curl -LO https://github.com/gittuf/gittuf/releases/download/v${VERSION}/gittuf_${VERSION}_${OS}_${ARCH}.pem

cosign verify-blob \
    --certificate gittuf_${VERSION}_${OS}_${ARCH}.pem \
    --signature gittuf_${VERSION}_${OS}_${ARCH}.sig \
    --certificate-identity-regexp https://github.com/gittuf/gittuf/.github/workflows/release.yml@refs/tags/v${VERSION} \
    --certificate-oidc-issuer https://token.actions.githubusercontent.com \
    gittuf_${VERSION}_${OS}_${ARCH}

sudo install gittuf_${VERSION}_${OS}_${ARCH} /usr/local/bin/gittuf
cd -
gittuf version
```

> [!NOTE]
> Please use release v0.1.0 or higher, as prior releases were created to
> test the release workflow.

Alternatively, gittuf can also be installed using `go install`.

To build from source, clone the repository and run `make`. This will also run
the test suite prior to installing gittuf. Note that Go 1.21 or higher is
necessary to build gittuf.

```bash
$ git clone https://github.com/gittuf/gittuf
$ cd gittuf
$ make
```

[The Update Framework (TUF)]: https://theupdateframework.io/
[other attacks]: https://ssl.engineering.nyu.edu/papers/torres_toto_usenixsec-2016.pdf
[contributing guide]: /CONTRIBUTING.md
[roadmap]: /docs/roadmap.md
[Open Source Security Foundation (OpenSSF)]: https://openssf.org/
[Supply Chain Integrity Working Group]: https://github.com/ossf/wg-supply-chain-integrity
[GoReleaser]: https://goreleaser.com/
[Sigstore]: https://www.sigstore.dev/
[cosign]: https://github.com/sigstore/cosign
