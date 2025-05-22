<img src="https://raw.githubusercontent.com/gittuf/community/bd8b367fa91fab0fddaa1943e0131e90e04e6b10/artwork/PNG/gittuf_horizontal-color.png" alt="gittuf logo" width="25%"/>

[![gittuf Verification](https://github.com/gittuf/gittuf/actions/workflows/gittuf-verify.yml/badge.svg)](https://github.com/gittuf/gittuf/actions/workflows/gittuf-verify.yml)
[![OpenSSF Best Practices](https://www.bestpractices.dev/projects/7789/badge)](https://www.bestpractices.dev/projects/7789)
![Build and Tests (CI)](https://github.com/gittuf/gittuf/actions/workflows/ci.yml/badge.svg)
[![Coverage Status](https://coveralls.io/repos/github/gittuf/gittuf/badge.svg)](https://coveralls.io/github/gittuf/gittuf)

gittuf is a platform-agnostic Git security system. The maintainers of a Git
repository can use gittuf to protect the contents of a Git repository from
unauthorized or malicious changes. Most significantly, gittuf’s policy controls
and enforcement is not tied to your source control platform (SCP) or “forge”,
meaning any developer can independently verify that a repository’s changes
followed the expected security policies. In other words, gittuf removes the
forge as a single point of trust in the software supply chain!

gittuf is a sandbox project at the [Open Source Security Foundation (OpenSSF)]
as part of the [Supply Chain Integrity Working Group].

## Current Status

gittuf is currently in beta. gittuf's metadata is versioned, and updates should
not require reinitializing a repository's gittuf policy. We recommend trying out
gittuf in addition to existing repository security mechanisms you may already be
using (e.g., forge security policies). We're actively seeking feedback from
users, please open an issue with any suggestions or bugs you encounter!

## Installation, Get Started, Get Involved

Take a look at the [get started guide] to learn how to install and try gittuf
out! Additionally, contributions are welcome, please refer to the [contributing
guide], our [roadmap], and the issue tracker for ways to get involved. In
addition, you can join the gittuf channel on the [OpenSSF Slack] and say hello! 

[contributing guide]: /CONTRIBUTING.md
[roadmap]: /docs/roadmap.md
[Open Source Security Foundation (OpenSSF)]: https://openssf.org/
[Supply Chain Integrity Working Group]: https://github.com/ossf/wg-supply-chain-integrity
[get started guide]: /docs/get-started.md
[OpenSSF Slack]: https://slack.openssf.org/
