<img src="https://raw.githubusercontent.com/gittuf/community/bd8b367fa91fab0fddaa1943e0131e90e04e6b10/artwork/PNG/gittuf_horizontal-color.png" alt="gittuf logo" width="25%"/>

[![OpenSSF Best Practices](https://www.bestpractices.dev/projects/7789/badge)](https://www.bestpractices.dev/projects/7789)
![Build and Tests (CI)](https://github.com/gittuf/gittuf/actions/workflows/ci.yml/badge.svg)
[![Coverage Status](https://coveralls.io/repos/github/gittuf/gittuf/badge.svg)](https://coveralls.io/github/gittuf/gittuf)

gittuf is a security layer for Git repositories. With gittuf, any developer who
can pull from a Git repository can independently verify that the repository's
security policies were followed. gittuf's policy, inspired by [The Update
Framework (TUF)], handles key management for all trusted developers in a
repository, allows for setting permissions for repository branches, tags, files,
etc., protects against [other attacks] Git is vulnerable to, and more — all
while being backwards compatible with GitHub, GitLab, etc.

gittuf is a sandbox project at the [Open Source Security Foundation (OpenSSF)]
as part of the [Supply Chain Integrity Working Group].

## Current Status

gittuf is currently in alpha. gittuf's metadata may have breaking changes,
meaning a repository's gittuf policy may have to be reinitialized from time to
time. As such, gittuf is currently not intended to be the primary mechanism for
enforcing a repository's security.

That said, we're actively seeking feedback from users. Take a look at the [get
started guide] to learn how to install and try gittuf out! Additionally,
contributions are welcome, please refer to the [contributing guide], our
[roadmap], and the issue tracker for ways to get involved.

## Installation & Get Started

See the [get started guide].

[The Update Framework (TUF)]: https://theupdateframework.io/
[other attacks]: https://ssl.engineering.nyu.edu/papers/torres_toto_usenixsec-2016.pdf
[contributing guide]: /CONTRIBUTING.md
[roadmap]: /docs/roadmap.md
[Open Source Security Foundation (OpenSSF)]: https://openssf.org/
[Supply Chain Integrity Working Group]: https://github.com/ossf/wg-supply-chain-integrity
[get started guide]: /docs/get-started.md
