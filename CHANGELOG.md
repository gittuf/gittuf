# Changelog

This file tracks the changes introduced by gittuf versions.

## v0.2.0

* Added support to RSL to find unskipped entries
* Added `Get*` functions to gitinterface to compartmentalize choice of Git
  library
* Added support in RSL and policy functions for RSL annotation entries
* Added recovery mode for policy verification workflow
* Added `go fmt` as Makefile target
* Updated length of refspecs slice to account for doubled entries
* Added support for merge commits in gitinterface
* Updated CLI to check if Git signing is viable to abort early
* Fixed bug in CLI that required an unnecessary signing key argument
* Fixed `clone`'s ability to handle trailing slashes
* Improved testing for in policy verification for delegations
* Added plumbing for better logging
* Updated various dependencies
* Updated installation instructions to include Sigstore verification of binaries

## v0.1.0

* Implemented reference state log (RSL)
* Added support for Git reference policies using RSL entry signatures
* Added support for file policies using commit signatures
* Added support for basic gittuf sync operations
