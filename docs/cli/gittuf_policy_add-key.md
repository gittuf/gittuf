## gittuf policy add-key

Add a trusted key to a policy file

### Synopsis

This command allows users to add a trusted key to the specified policy file. By default, the main policy file is selected. Note that the keys can be specified from disk, from the GPG keyring using the "gpg:<fingerprint>" format, or as a Sigstore identity as "fulcio:<identity>::<issuer>".

```
gittuf policy add-key [flags]
```

### Options

```
      --authorize-key stringArray   authorized public key for rule
  -h, --help                        help for add-key
      --policy-name string          policy file to add rule to (default "targets")
```

### Options inherited from parent commands

```
  -k, --signing-key string   signing key to use to sign policy file
      --verbose              enable verbose logging
```

### SEE ALSO

* [gittuf policy](gittuf_policy.md)	 - Tools to manage gittuf policies

