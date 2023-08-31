# gittuf

[![OpenSSF Best Practices](https://www.bestpractices.dev/projects/7789/badge)](https://www.bestpractices.dev/projects/7789)

gittuf provides a security layer for Git using some concepts introduced by [The
Update Framework (TUF)]. Among other features, gittuf handles key management for
all developers on the repository, allows you to set permissions for repository
branches, tags, files, etc., lets you use new cryptographic algorithms (SHA256,
etc.), protects against [other attacks] Git is vulnerable to, and more — all
while being backwards compatible with GitHub, GitLab, etc.

## Current Status

gittuf is currently approaching an alpha release. It is NOT intended for use in
a production system or repository. Contributions are welcome, please refer to
the [contributing guide]. Some of the features listed above are being actively
developed, please refer to the [roadmap] and the issue tracker for more details.

## Installation

gittuf requires Go 1.20 or higher.

The tool can be installed using `go install` as follows:

```bash
$ go install github.com/gittuf/gittuf@latest
```

Alternatively, you can clone the repository and run `make`. This will also run
the test suite prior to installing gittuf.

```bash
$ git clone https://github.com/gittuf/gittuf
$ cd gittuf
$ make
```

[The Update Framework (TUF)]: https://theupdateframework.io/
[other attacks]: https://ssl.engineering.nyu.edu/papers/torres_toto_usenixsec-2016.pdf
[contributing guide]: /CONTRIBUTING.md
[roadmap]: /docs/roadmap.md
