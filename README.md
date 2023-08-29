# gittuf

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

[The Update Framework (TUF)]: https://theupdateframework.io/
[other attacks]: https://ssl.engineering.nyu.edu/papers/torres_toto_usenixsec-2016.pdf
[contributing guide]: /CONTRIBUTING.md
[roadmap]: /docs/roadmap.md
