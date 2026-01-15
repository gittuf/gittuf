# Read Permissions in gittuf

## Metadata

* **Number:** TBD
* **Title:** Read Permissions in gittuf
* **Implemented:** No
* **Withdrawn/Rejected:** No
* **Sponsors:** Patrick Zielinski (patzielinski)
* **Related GAPs:**
* **Last Modified:** September 9, 2025

## Abstract

gittuf supports defining a write-access-control policy for a repository, where
changes to the repository may be independently verified against the policy.

## Specification

TODO

## Motivation

Sometimes, data must not be readable by everyone with pull access to the Git
repository the data is stored on. As Git does not support restricting read
access to specific data on Git repositories, the current solution is to avoid
storing said data on Git repositories at all, and instead sotre it in secrets
vaults or similar.

TODO: Expand

## Reasoning

TODO

## Backwards Compatibility

TODO

## Security

TODO

## Prototype Implementation

A prototype is currently under construction and available at
https://github.com/patzielinski/gittuf.

## Implementation

TODO

## Changelog

TODO

## Acknowledgements

TODO

## References

TODO
