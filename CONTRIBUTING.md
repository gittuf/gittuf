# Contributing Guide

Contributions to gittuf can be of several types:
* changes to the [design document](/docs/design-document.md) or
  [gittuf Augmentation Proposals (GAPs)](/docs/gaps/README.md) stored in the
  `docs/` folder
* code changes for bug fixes, new features, documentation, and other
  enhancements to the implementation
* new issues or feature requests

[Join our community](https://github.com/gittuf/community/?tab=readme-ov-file#join-us)
to get started!

## Contributor Workflow

When submitting changes to the gittuf docs or implementation, contributors must
open a GitHub pull request to the repository. If a proposed change is a
significant deviation from gittuf's [design document](/docs/design-document.md),
a [GAP](/docs/gaps/README.md) may be necessary. When in doubt, contributors are
advised to file an issue in the repository for the
[maintainers](MAINTAINERS.txt) to determine the best way forward.

gittuf uses the NYU Secure Systems Lab [development
workflow](https://github.com/secure-systems-lab/lab-guidelines/blob/master/dev-workflow.md).
Pull requests must include tests for the changes in behavior they introduce.
They are reviewed by one or more [maintainers](MAINTAINERS.txt) and undergo
automated testing such as (but not limited to):
* Unit and build testing
* Static analysis using linters
* Developer Certificate of Origin (DCO) check

## Dependencies Policy

As third-party dependencies vary in code quality compared to gittuf, and can
introduce issues, their use in gittuf is regulated by this policy. This policy
applies to all gittuf contributors and all third-party packages used in the
gittuf project.

### Policy

gittuf contributors must follow these guidelines when consuming third-party
packages:

- Only use third-party packages that are necessary for the functionality of
  gittuf.
- Use the latest version of all third-party packages whenever possible.
- Avoid using third-party packages that are known to have security
  vulnerabilities.
- Pin all third-party packages to specific versions in the gittuf codebase.
- Use a dependency management tool, such as Go modules, to manage third-party
  dependencies.

### Procedure

When adding a new third-party package to gittuf, maintainers must follow these
steps:

1. Evaluate the need for the package. Is it necessary for the functionality of
   gittuf?
2. Research the package. Is it well-maintained? Does it have a good reputation?
3. Choose a version of the package. Use the latest version whenever possible.
4. Pin the package to the specific version in the gittuf codebase.
5. Update the gittuf documentation to reflect the new dependency.

### Enforcement

This policy is enforced by the gittuf maintainers. Maintainers are expected to
review each other's as well as contributors' code changes to ensure that they 
comply with this policy.

### Exceptions

Exceptions to this policy may be granted by the gittuf TSC on a case-by-case
basis.

## Other Guidelines

Contributors to gittuf must abide by the project's
[code of conduct](https://github.com/gittuf/community/blob/main/CODE-OF-CONDUCT.md).
Any questions regarding the gittuf community's governance and code of conduct
may be directed to the project's
[Technical Steering Committee](https://github.com/gittuf/community/blob/main/TECHNICAL-STEERING-COMMITTEE.md).
