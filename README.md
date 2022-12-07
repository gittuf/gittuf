# gittuf

The security layer for Git, built using
[The Update Framework (TUF)](https://theupdateframework.io/).

## Current Status

gittuf is currently a pre-pre-alpha. It is NOT intended for use in anything
remotely resembling a production system or repository. Contributions are
welcome!

## Use

Build and install gittuf using the [Makefile](./Makefile). Note that gittuf has
an implicit dependency on the Git binary installed on the system. Install Git
from your preferred package manager.

Currently, gittuf supports
[ED25519 keys only](https://github.com/adityasaky/gittuf/issues/5) and expects
them to be in the
[securesystemslib format](https://github.com/secure-systems-lab/securesystemslib/blob/master/securesystemslib/formats.py#L316-L323).
