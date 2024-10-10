# Vendored secure-systems-lab/go-securesystemslib

Issue: https://github.com/gittuf/gittuf/issues/266

## dsse

The dsse package has been vendored to experimentally add support for DSSE
signature extensions. We're starting with support for Sigstore, and once we land
on a reusable interface for extensions, we can upstream this to
go-securesystemslib.
