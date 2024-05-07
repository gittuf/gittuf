<img src="https://raw.githubusercontent.com/gittuf/community/bd8b367fa91fab0fddaa1943e0131e90e04e6b10/artwork/PNG/gittuf_horizontal-color.png" alt="gittuf logo" width="25%"/>

[![OpenSSF Best Practices](https://www.bestpractices.dev/projects/7789/badge)](https://www.bestpractices.dev/projects/7789)
![Build and Tests (CI)](https://github.com/gittuf/gittuf/actions/workflows/ci.yml/badge.svg)

gittuf provides a security layer for Git using some concepts introduced by [The
Update Framework (TUF)]. Among other features, gittuf handles key management for
all developers on the repository, allows you to set permissions for repository
branches, tags, files, etc., protects against [other attacks] Git is vulnerable
to, and more — all while being backwards compatible with GitHub, GitLab, etc.

gittuf is a sandbox project at the [Open Source Security Foundation (OpenSSF)]
as part of the [Supply Chain Integrity Working Group].

## Current Status

gittuf is currently in alpha. It is NOT intended for use in a production system
or repository. Contributions are welcome, please refer to the [contributing
guide]. Some of the features listed above are being actively developed, please
refer to the [roadmap] and the issue tracker for more details.

## Installation & Get Started

See the [get started guide].

[The Update Framework (TUF)]: https://theupdateframework.io/
[other attacks]: https://ssl.engineering.nyu.edu/papers/torres_toto_usenixsec-2016.pdf
[contributing guide]: /CONTRIBUTING.md
[roadmap]: /docs/roadmap.md
[Open Source Security Foundation (OpenSSF)]: https://openssf.org/
[Supply Chain Integrity Working Group]: https://github.com/ossf/wg-supply-chain-integrity
[get started guide]: /docs/get-started.md
