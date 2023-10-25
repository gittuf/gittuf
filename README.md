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

gittuf is currently approaching an alpha release. It is NOT intended for use in
a production system or repository. Contributions are welcome, please refer to
the [contributing guide]. Some of the features listed above are being actively
developed, please refer to the [roadmap] and the issue tracker for more details.

## Installation

gittuf requires Go 1.21 or higher.

Currently, gittuf needs to be built from source. After cloning the repository,
run `make`. This will also run the test suite prior to installing gittuf.

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
