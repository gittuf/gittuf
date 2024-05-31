# Teams support in gittuf
gittuf has prototype-level support for teams, as defined in [TUF's
TAP-3](https://github.com/theupdateframework/taps/blob/master/tap3.md).

## Existing CLI
The existing workflow to manage rules via the CLI is through `add-rule`,
`update-rule`, and `remove-rule`. For backwards compatibility, these commands
are preserved even with the underlying metadata changed. To be compatible with
the new underlying metadata format, `add-rule` and `update-rule` now create a
placeholder "team" whenever they are invoked - the name of this "team" is
"Single Role".

## New Teams CLI
In order to support creating teams, two new commands were added to the CLI:
`add-rule-teams` and `update-rule-teams`. As the underlying rule definition does
not matter to `remove-rule`, that command remains the same and works for both
legacy and teams workflows.

At the moment, roles can be specified via a JSON file, whose format is as
follows:

```json
[
  {
    "name" : "rolename",
    "keys" : ["path-to-key1", "path-to-key2"],
    "threshold" : 1
  },
  {
    "name" : "rolename",
    "keys" : ["path-to-key1", "path-to-key2"],
    "threshold" : 1
  }
]
```

Authorized keys can be specified from disk, from the GPG keyring using the
`"gpg:<fingerprint>"` format, or as a Sigstore identity as
`"fulcio:<identity>::<issuer>"`. For on-disk keys, make sure to specify the full
path to the key file.