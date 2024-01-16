# Vendored secure-systems-lab/go-securesystemslib

We have temporarily vendored go-securesystemslib's signerverifier package, a
dependency we use for signature creation / verification workflows. The reason
for this is to update go-securesystemslib with support for standard encoding
formats for on-disk keys. There is a parallel effort to improve the library that
may either use the changes here or provide alternative options with similar
intent.

The library was vendored with some new APIs to create signers for PEM encoded
keys. It also removes the "private" field in the key serialization, as we don't
want want to store private keys in a custom format anywhere. In addition, SSH
key parsing support has been added, as those are common in Git workflows.
Finally, to support legacy key formats in gittuf, the ED25519 signerverifier's
fields have been made public.

Note that @adityasaky, maintainer of gittuf, is also a maintainer of
go-securesystemslib, and is part of the improvement effort. After it is
complete, this copy will be taken out.

Issue: https://github.com/gittuf/gittuf/issues/266
