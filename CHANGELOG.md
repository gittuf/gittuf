# Changelog

This file tracks the changes introduced by gittuf versions.

## v0.7.0

- Added support for metadata signing using Sigstore (currently `GITTUF_DEV`
  only)
- Removed use of legacy custom securesystemslib key formats in gittuf's tests
- Removed vendored signerverifier library
- Unified SSH signature verification for Git commits and tags
- Refactored `policy` and `tuf` packages to support versioning policy metadata
- Updated various dependencies and CI workflows

## v0.6.2

- Added `git-remote-gittuf` to the release workflow's pre-built artifacts
- Updated CI workflow dependency

## v0.6.1

- Added a counter to RSL entries to support persistent caching
- Added experimental support for signature extensions to vendored DSSE library
- Refactored `GetLatestReferenceEntry` RSL API
- Fixed Makefile build on Windows
- Moved `update-root-threshold` and `update-policy-threshold` out of developer
  mode
- Fixed issue with git-remote-gittuf using the wrong transport when fetching the
  RSL
- Fixed issue with git-remote-gittuf when explicitly pushing the RSL
- Fixed issue with git-remote-gittuf and `curl` fetches and pushes on Windows
- Increased testing coverage of `policy` and `gitinterface`
- Improved documentation for getting started with gittuf, especially on Windows
  platforms
- Added copyright notices to code files
- Updated various dependencies and CI workflows

## v0.6.0

- Added command to reorder policy rules
- Added support for older Git versions
- Added support for GitHub pull request approval attestations
- Added support for using enterprise GitHub instances
- Added caching for the RSL APIs `GetEntry` and `GetParentForEntry`
- Added parallelization for some unit tests
- Removed some deprecated flows such as `FindPublicKeysForPath` and refactored
  verification APIs
- Added CodeQL scanning for the repository
- Updated various dependencies and CI workflows

## v0.5.2

- Fixed issue with git-remote-gittuf when force pushing 
- Fixed issue with git-remote-gittuf not fetching RSL before adding new entries
- Updated various dependencies

## v0.5.1

- Updated release workflow to support GoReleaser v2

## v0.5.0

- Added support for `ssh-keygen` based signer and verifier
- Added support for overriding reference name when local and remote reference
  names differ
- Added initial (alpha) implementation of git-remote-gittuf
- Added command to display RSL
- Added support for automatically skipping RSL entries that point to rebased
  commits
- Updated policy verification pattern matching to use `fnmatch`
- Updated to use Git binary for various operations on underlying repository
- Updated various dependencies and CI workflows
- Updated docs to make command snippets easier to copy
- Removed extraneous fields from gittuf policy metadata
- Removed `verify-commit` and `verify-tag` workflows in favor of `verify-ref`
  (BREAKING CHANGE)
- Governance: added Patrick Zielinski and Neil Naveen as gittuf maintainers

## v0.4.0

- Added support for `policy-staging` for sequential signing of metadata to meet
  a threshold
- Added support for minimum required signatures for rules
- Added support for profiling with pprof
- Added `--from-entry` to `verify-ref`
- Added debug statements for `--verbose` flag
- Added caching of verifiers for each verified namespace (reference or file
  path) to avoid repeated searches of the same policy state
- Added separated `add-rule` and `update-rule` workflows for policy
- Added dogfooding plan
- Added CI workflows for phase 1 of dogfooding
- Added OpenSSF Scorecard for the repository
- Updated policy to require each rule name to be unique across all rule files
- Updated file rules verification to use same policy as branch protection rules
  verification
- Update reference authorization attestations to use merge tree for the change
  being authorized
- Updated design document with definitions and a diagram
- Updated tag verification to check the tag's RSL entry points to either the tag
  object or the tag's target object
- Updated roadmap to indicate status for each item
- Updated minimum Go version to 1.22
- Updated pointer to gittuf community details
- Updated various dependencies and CI workflows

## v0.3.0

- Added check to prevent duplicate RSL entries for the same ref and target
- Added a formal developer mode for new early-stage gittuf features
- Added early support for attestations with one type for approving reference
  changes (developer mode only)
- Added support for gittuf-specific Git hooks with a pre-push hook to fetch /
  create / push RSL entries
- Updated `verify-ref` to perform full verification by default (BREAKING CHANGE)
- Updated identification of trusted keys in policy to support varying threshold
  values between delegations
- Added verification tests for delegated policies
- Added root key management commands to the CLI
- Added command to list rules in gittuf policy
- Added support for standard encoding of private and public keys
- Added support for verifying SSH Git commit and tag signatures
- Added check for cycles when walking policy graph during verification
- Added autogenerated CLI docs
- Removed file rule verification when no file rules exist in the policy for
  efficiency
- Added command to sign existing policy file with no other changes
- Added get started guide and gittuf logo to docs
- Removed CLI usage message for gittuf errors
- Updated various dependencies

## v0.2.0

- Added support to RSL to find unskipped entries
- Added `Get*` functions to gitinterface to compartmentalize choice of Git
  library
- Added support in RSL and policy functions for RSL annotation entries
- Added recovery mode for policy verification workflow
- Added `go fmt` as Makefile target
- Updated length of refspecs slice to account for doubled entries
- Added support for merge commits in gitinterface
- Updated CLI to check if Git signing is viable to abort early
- Fixed bug in CLI that required an unnecessary signing key argument
- Fixed `clone`'s ability to handle trailing slashes
- Improved testing for in policy verification for delegations
- Added plumbing for better logging
- Updated various dependencies
- Updated installation instructions to include Sigstore verification of binaries

## v0.1.0

- Implemented reference state log (RSL)
- Added support for Git reference policies using RSL entry signatures
- Added support for file policies using commit signatures
- Added support for basic gittuf sync operations
