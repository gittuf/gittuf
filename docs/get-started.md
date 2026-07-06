# Get Started

This guide presents a quick primer to using gittuf. Note that gittuf is
currently in beta, so if you encounter any bugs, we encourage you to [open an
issue].

## Install gittuf

The instructions to install gittuf, either from prebuilt binaries, or from
source, are available on [gittuf.dev - Quickstart]. After gittuf has been
installed, you may continue with the instructions below.

## A Note on Git References

gittuf follows Git’s order for resolving reference names. This means that if you
have ambiguous reference names, such as the tag `main` and the branch `main`,
gittuf will perform operations on the **tag**, not the branch!

We suggest you use fully-qualified reference names to avoid ambiguity and ensure
that you are operating on the correct reference. In our example above, the
fully-qualified reference name of the tag `main` would be `refs/tags/main`,
while the branch would be `refs/heads/main`.

## Create keys

First, create some keys that are used for the gittuf root of trust, policies, as
well as for commits created while following this guide.

> [!NOTE]
> If running on Windows, do not use the `-N ""` flag in the `ssh-keygen`
> commands.
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
repository.

If you are using gittuf on an existing repository, please note that gittuf
currently does not make any claims about the contents of the repository before
gittuf was set up. This means that all contents of the repository are assumed to
be trusted, and only changes after gittuf initialization will be scrutinized
according to the policy. To use gittuf on an existing repository, skip to
[Initialize gittuf].

If you would like to use gittuf on a new repository, simply initialize the
repository and gittuf's root of trust metadata using the key.

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
Add a trusted person to the policy file. Then, use the policy key to initialize
a policy and add a rule protecting the `main` branch.

```bash
gittuf policy add-person -k ../keys/policy --person-ID developer --public-key ../keys/developer.pub
gittuf policy add-rule -k ../keys/policy --rule-name protect-main --rule-pattern git:refs/heads/main --authorize developer
```

Note that `add-key` can also be used to specify a GPG key or a [Sigstore]
identity for use with [gitsign].

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

gittuf includes two main ways to push and fetch the policy and RSL references.
You may use the `gittuf sync` command to synchronize changes with the remote
automatically. You may also instead use the [gittuf transport], which handles
the synchronization of gittuf metadata transparently upon standard Git pushes
and pulls, without needing to explicitly invoke gittuf.

If you prefer to manually synchronize references, Git can be used to keep
gittuf's references updated.

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

[gittuf.dev - Quickstart]: https://gittuf.dev/quickstart
[Sigstore]: https://www.sigstore.dev/
[cosign]: https://github.com/sigstore/cosign
[gitsign]: https://github.com/sigstore/gitsign
[GoReleaser]: https://goreleaser.com/
[#220]: https://github.com/gittuf/gittuf/issues/220
[CLI docs]: /docs/cli/gittuf.md
[open an issue]: https://github.com/gittuf/gittuf/issues/new/choose
[dogfooding]: /docs/dogfood.md
[Initialize gittuf]: #initialize-gittuf
[gittuf transport]: /internal/git-remote-gittuf/README.md
