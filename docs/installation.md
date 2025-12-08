# Installing gittuf

There are two ways to install gittuf: from pre-built binaries, or from source.
Once you have followed one of these methods, see the [get-started.md]

## Install gittuf using pre-built binaries

This repository provides pre-built binaries that are signed and published using
[GoReleaser]. The signatures for these binaries are generated using [Sigstore],
using the release workflow's identity. Make sure you have [cosign] installed on
your system, then you will be able to securely download and verify the gittuf
release:

### Unix-like operating systems

If you use Linux, macOS, or FreeBSD, copy the following script into your
terminal to install gittuf. Your distribution's package manager may have gittuf,
but it will likely be an older version.

```sh
`# Modify these values as necessary.
# One of: amd64, arm64
ARCH=amd64
# One of: linux, darwin, freebsd
OS=linux
# See https://github.com/gittuf/gittuf/releases for the latest version
VERSION=0.12.0
cd $(mktemp -d)

curl -LO https://github.com/gittuf/gittuf/releases/download/v${VERSION}/gittuf_${VERSION}_${OS}_${ARCH}
curl -LO https://github.com/gittuf/gittuf/releases/download/v${VERSION}/gittuf_${VERSION}_${OS}_${ARCH}.sig
curl -LO https://github.com/gittuf/gittuf/releases/download/v${VERSION}/gittuf_${VERSION}_${OS}_${ARCH}.pem

cosign verify-blob \
    --certificate gittuf_${VERSION}_${OS}_${ARCH}.pem \
    --signature gittuf_${VERSION}_${OS}_${ARCH}.sig \
    --certificate-identity https://github.com/gittuf/gittuf/.github/workflows/release.yml@refs/tags/v${VERSION} \
    --certificate-oidc-issuer https://token.actions.githubusercontent.com \
    gittuf_${VERSION}_${OS}_${ARCH}

sudo install gittuf_${VERSION}_${OS}_${ARCH} /usr/local/bin/gittuf
cd -
gittuf version`
```

### Windows

If you use Windows, you can choose between the Winget package manager and
manually downloading gittuf.

#### Winget

gittuf can be installed on Windows from winget, provided winget is installed
on the system:

```powershell
winget install gittuf
```

#### Manual installation

Copy and paste these commands in PowerShell to install gittuf. Please remember
to change the version number (0.12.0 in this example) and architecture
(amd64 in this example) according to your use-case and system.

```powershell
curl "https://github.com/gittuf/gittuf/releases/download/v0.12.0/gittuf_0.12.0_windows_amd64.exe" -O "gittuf_0.12.0_windows_amd64.exe"
curl "https://github.com/gittuf/gittuf/releases/download/v0.12.0/gittuf_0.12.0_windows_amd64.exe.sig" -O "gittuf_0.12.0_windows_amd64.exe.sig"
curl "https://github.com/gittuf/gittuf/releases/download/v0.12.0/gittuf_0.12.0_windows_amd64.exe.pem" -O "gittuf_0.12.0_windows_amd64.exe.pem"

cosign verify-blob --certificate gittuf_0.12.0_windows_amd64.exe.pem --signature gittuf_0.12.0_windows_amd64.exe.sig --certificate-identity https://github.com/gittuf/gittuf/.github/workflows/release.yml@refs/tags/v0.12.0 --certificate-oidc-issuer https://token.actions.githubusercontent.com gittuf_0.12.0_windows_amd64.exe
```

The gittuf binary is now verified on your system. You can run it from the
terminal as `gittuf.exe` from this directory, or add it to your PATH as desired.

## Building from source

If you have already downloaded and installed gittuf using the above
instructions, skip to the [getting started instructions]. You need only follow
the below instructions should you wish to build and install gittuf from source.

> [!NOTE]
> `make` needs to be installed manually on Windows as it is not packaged with
> the OS. The easiest way to install `make` on Windows is to use the
> `ezwinports.make` package: Simply type `winget install ezwinports.make`
> in PowerShell.
> You can also install it from the [GNU website] or the [chocolatey] package
> manager.

Before building from source, please ensure that your Go environment is correctly
set up. You can do this by following the official [Go installation
instructions]. If you encounter any issues when building, make sure your
`GOPATH` and `PATH` are configured correctly.

To build from source, clone the repository and run `make`. This will also run
the test suite prior to installing gittuf. Note that Go 1.24 or higher is
necessary to build gittuf.

```sh
git clone https://github.com/gittuf/gittuf
cd gittuf
make
```

This will automatically put `gittuf` in the `GOPATH` as configured.

[Sigstore]: https://www.sigstore.dev/
[cosign]: https://github.com/sigstore/cosign
[GoReleaser]: https://goreleaser.com/
[open an issue]: https://github.com/gittuf/gittuf/issues/new/choose
[dogfooding]: /docs/dogfood.md
[GNU website]: https://gnuwin32.sourceforge.net/packages/make.htm
[chocolatey]: https://community.chocolatey.org/packages/make
[official Go guide for Windows]: https://go.dev/wiki/SettingGOPATH#
[Go installation instructions]: https://go.dev/doc/install
[getting started instructions]: /docs/get-started.md
