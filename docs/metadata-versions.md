# gittuf Metadata Versions

Over time, gittuf metadata may change to support new features, for example. As
existing metadata cannot be directly rewritten, a version identifier is present
to signal to gittuf what to expect when parsing the underlying data.

The version of the metadata can be found in the JSON property
`metadata_version`, located in the root and targets metadata of the TUF metadata
used in gittuf.

The format follows the [Semantic Versioning 2.0.0
(semver)](https://semver.org/spec/v2.0.0.html) specification. As the metadata
version does not directly correlate with the version number of gittuf, the
metadata version starts from major version `100` (i.e. `100.0.0`).

## Version `100.x.x` (Legacy)

This version refers to any metadata before this versioning system arose.
Metadata without the `metadata_version` property present is treated as version
`100.0.0`.

TODO: Describe metadata here.

## Version `101.x.x` (Teams) (TBD)

This version adds support for teams, and is a breaking change as the underlying
TUF metadata was upgraded to comply with TAP 3.