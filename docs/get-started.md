# Get Started

This guide presents a quick primer to using gittuf. Note that gittuf is
currently in alpha, and it is not intended for use in a production repository.

## Install gittuf

> [!NOTE]
> Please use release v0.1.0 or higher, as prior releases were created to
> test the release workflow.

**Pre-built binaries:** This repository provides pre-built binaries that are
signed and published using [GoReleaser]. The signature for these binaries are
generated using [Sigstore], using the release workflow's identity. Make sure you
have [cosign] installed on your system, then you will be able to securely
download and verify the gittuf release:

> [!NOTE]
> For `windows`, the `.exe` extension needs to be included for the binary (as `filename.exe`),
> signature (as `filename.exe.sig`) and certificate (as `filename.exe.sig`) files.
> Similarly, `sudo install` needs to be changed to `Start-Process` along with the destination path. Explicit instructions are listed below.

### Unix-based operating systems 
```sh
# Modify these values as necessary.
# One of: amd64, arm64
ARCH=amd64
# One of: linux, darwin, freebsd
OS=linux
# See https://github.com/gittuf/gittuf/releases for the latest version
VERSION=0.4.0
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
```sh
# Modify these values as necessary.
# One of: amd64, arm64
ARCH=amd64
OS=windows
# See https://github.com/gittuf/gittuf/releases for the latest version
VERSION=0.5.2

Push-Location # this saves the pwd to the stack

curl -LO https://github.com/gittuf/gittuf/releases/download/v${VERSION}/gittuf_${VERSION}_${OS}_${ARCH}.exe
curl -LO https://github.com/gittuf/gittuf/releases/download/v${VERSION}/gittuf_${VERSION}_${OS}_${ARCH}.exe.sig
curl -LO https://github.com/gittuf/gittuf/releases/download/v${VERSION}/gittuf_${VERSION}_${OS}_${ARCH}.exe.pem

cosign verify-blob --certificate gittuf_${VERSION}_${OS}_${ARCH}.exe.pem --signature gittuf_${VERSION}_${OS}_${ARCH}.exe.sig --certificate-identity https://github.com/gittuf/gittuf/.github/workflows/release.yml@refs/tags/v${VERSION} --certificate-oidc-issuer https://token.actions.githubusercontent.com gittuf_${VERSION}_${OS}_${ARCH}

Start-Process -FilePath ".\gittuf_${VERSION}_windows_${ARCH}.exe" -Verb RunAs

Pop-Location # pushes pwd from the stack to return to the directory you were in before
gittuf version
```

### Building from source
To build from source, clone the repository and run
`make`. This will also run the test suite prior to installing gittuf. Note that Go 1.22 or higher is necessary to build gittuf.

> [!NOTE]
> `make` needs to be installed externally on Windows, it is not packaged with the OS. You may install it from [chocolatey] or from the [GNU website].

```sh
git clone https://github.com/gittuf/gittuf
cd gittuf
make
```

## Create keys

First, create some keys that are used for the gittuf root of trust, policies, as
well as for commits created while following this guide.

> [!NOTE]
> If running on Windows, do not use the `-N ""` flag for the `ssh-keygen` commands.
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
gittuf policy add-rule -k ../keys/policy --rule-name protect-main --rule-pattern git:refs/heads/main --authorize-key ../keys/developer.pub
```

Note that `--authorize-key` can also be used to specify a GPG key or a
[Sigstore] identity for use with [gitsign]. However, we're using SSH keys
throughout in this guide, as gittuf policy metadata currently cannot be signed
using GPG and Sigstore (see [#229]).

After adding the required policies, _apply_ them from the policy-staging area.
This means the policy will be applicable henceforth.

```bash
gittuf policy apply
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
gittuf rsl record main
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
