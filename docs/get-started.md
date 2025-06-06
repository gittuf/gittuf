# Get Started

This guide presents a quick primer to using gittuf. Note that gittuf is
currently in alpha, and it is not intended for use in a production repository.

## Install gittuf using pre-built binaries

> [!NOTE]
> Please use release v0.1.0 or higher, as prior releases were created to
> test the release workflow.

This repository provides pre-built binaries that are signed and published using
[GoReleaser]. The signature for these binaries are generated using [Sigstore],
using the release workflow's identity. Make sure you have [cosign] installed on
your system, then you will be able to securely download and verify the gittuf
release:

### Unix-like operating systems

```sh
# Modify these values as necessary.
# One of: amd64, arm64
ARCH=amd64
# One of: linux, darwin, freebsd
OS=linux
# See https://github.com/gittuf/gittuf/releases for the latest version
VERSION=0.8.0
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
gittuf version
```

### Windows

#### Winget

gittuf can be installed on Windows from winget, provided winget is installed
on the system:

```powershell
winget install gittuf
```

#### Manual installation

Copy and paste these commands in PowerShell to install gittuf. Please remember
to change the version number (0.8.0 in this example) and architecture
(amd64 in this example) according to your use-case and system.

```powershell
curl "https://github.com/gittuf/gittuf/releases/download/v0.8.0/gittuf_0.8.0_windows_amd64.exe" -O "gittuf_0.8.0_windows_amd64.exe"
curl "https://github.com/gittuf/gittuf/releases/download/v0.8.0/gittuf_0.8.0_windows_amd64.exe.sig" -O "gittuf_0.8.0_windows_amd64.exe.sig"
curl "https://github.com/gittuf/gittuf/releases/download/v0.8.0/gittuf_0.8.0_windows_amd64.exe.pem" -O "gittuf_0.8.0_windows_amd64.exe.pem"

cosign verify-blob --certificate gittuf_0.8.0_windows_amd64.exe.pem --signature gittuf_0.8.0_windows_amd64.exe.sig --certificate-identity https://github.com/gittuf/gittuf/.github/workflows/release.yml@refs/tags/v0.8.0 --certificate-oidc-issuer https://token.actions.githubusercontent.com gittuf_0.8.0_windows_amd64.exe
```

The gittuf binary is now verified on your system. You can run it from the terminal
as `gittuf.exe` from this directory, or add it to your PATH as desired.

## Building from source

> [!NOTE]
> `make` needs to be installed manually on Windows as it is not packaged with
> the OS. The easiest way to install `make` on Windows is to use the
> `ezwinports.make` package: Simply type `winget install ezwinports.make`
> in PowerShell.
> You can also install it from the [GNU website] or the [chocolatey] package manager.

Before building from source, please ensure that your Go environment is correctly
set up. You can do this by following the official [Go installation
instructions]. If you encounter any issues when building, make sure your
`GOPATH` and `PATH` are configured correctly.

To build from source, clone the repository and run `make`. This will also run
the test suite prior to installing gittuf. Note that Go 1.23 or higher is
necessary to build gittuf.

```sh
git clone https://github.com/gittuf/gittuf
cd gittuf
make
```

This will automatically put `gittuf` in the `GOPATH` as configured.

## Create keys

First, create some keys that are used for the gittuf root of trust, policies, as
well as for commits created while following this guide.

> [!NOTE]
> If running on Windows, do not use the `-N ""` flag in the `ssh-keygen` commands.
> Instead, enter an empty passphrase when prompted.

```bash
mkdir gittuf-get-started && cd gittuf-get-started
mkdir keys && cd keys
ssh-keygen -q -t ecdsa -N "" -f root
ssh-keygen -q -t ecdsa -N "" -f policy
ssh-keygen -q -t ecdsa -N "" -f developer
```

## Create a Git repository

gittuf can be used with either a brand new repository or with an existing
repository. Here, we assume gittuf is being deployed with a fresh repository.
Initialize the repository and gittuf's root of trust metadata using the
key.

```bash
cd .. && mkdir repo && cd repo
git init -q -b main
git config --local gpg.format ssh
git config --local user.signingkey ../keys/developer
```

## Initialize gittuf

Initialize gittuf's root of trust metadata.

```bash
gittuf trust init -k ../keys/root
```

After that, add a key for the primary policy. gittuf allows users to specify
rules in one or more policy files. The primary policy file (called `targets`,
from TUF) must be signed by keys specified in the root of trust.

```bash
gittuf trust add-policy-key -k ../keys/root --policy-key ../keys/policy.pub
gittuf policy init -k ../keys/policy --policy-name targets
```
Then, use the policy key to initialize a policy and add a rule protecting the
`main` branch.

```bash
gittuf policy add-key -k ../keys/policy --public-key ../keys/developer.pub
gittuf policy add-rule -k ../keys/policy --rule-name protect-main --rule-pattern git:refs/heads/main --authorize-key ../keys/developer.pub
```

Note that `add-key` can also be used to specify a GPG key or a [Sigstore]
identity for use with [gitsign]. However, we're using SSH keys throughout in
this guide, as gittuf policy metadata currently cannot be signed using GPG (see
[#229]). Also, `--authorize-key` in `gittuf policy add-rule` may return a
deprecation warning. This guide will be updated with the new `--authorize` flag
in its place.

Next, _stage_ the policies into the policy-staging area. The policy-staging
area is useful for sharing changes to policies that must not be used yet.

```bash
gittuf policy stage --local-only
```

After committing the policies, _apply_ them from the policy-staging area.  This
means the policy will be applicable henceforth.

```bash
gittuf policy apply --local-only
```

## Making repository changes

You can make changes in the repository using standard Git workflows. However,
changes to Git references (i.e., branches and tags) must be recorded in gittuf's
reference state log (RSL). Currently, this must be executed manually or using a
pre-push hook (see `gittuf add-hook -h` for more information about adding the
hook and [#220] for planned gittuf and Git command compatibility).

```bash
echo "Hello, world!" > README.md
git add . && git commit -q -S -m "Initial commit"
gittuf rsl record main --local-only
```

## Verifying policy

gittuf allows for verifying rules for Git references and files.

```sh
gittuf verify-ref --verbose main
```

## Communicating with a remote

gittuf includes helpers to push and fetch the policy and RSL references.
However, there are some known issues (see [#328]) with these commands. In the
meantime, Git can be used to keep gittuf's references updated.

```sh
git push <remote> refs/gittuf/*
git fetch <remote> refs/gittuf/*:refs/gittuf/*
```

## Verify gittuf itself

You can also verify the state of the gittuf source code repository with gittuf
itself. For more information on verifying gittuf with gittuf, visit the
[dogfooding] document.

## Conclusion

This is a very quick primer to gittuf! Please take a look at gittuf's [CLI docs]
to learn more about using gittuf. If you find a bug, please [open an issue] on
the gittuf repository.

[Sigstore]: https://www.sigstore.dev/
[cosign]: https://github.com/sigstore/cosign
[gitsign]: https://github.com/sigstore/gitsign
[GoReleaser]: https://goreleaser.com/
[#276]: https://github.com/gittuf/gittuf/issues/276
[#229]: https://github.com/gittuf/gittuf/issues/229
[#220]: https://github.com/gittuf/gittuf/issues/220
[#328]: https://github.com/gittuf/gittuf/issues/328
[CLI docs]: /docs/cli/gittuf.md
[open an issue]: https://github.com/gittuf/gittuf/issues/new/choose
[dogfooding]: /docs/dogfood.md
[GNU website]: https://gnuwin32.sourceforge.net/packages/make.htm
[chocolatey]: https://community.chocolatey.org/packages/make
[official Go guide for Windows]: https://go.dev/wiki/SettingGOPATH#
[Go installation instructions]: https://go.dev/doc/install
