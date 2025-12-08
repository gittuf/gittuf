# Get Started

This guide presents a quick primer to using gittuf. Note that gittuf is
currently in beta, so if you encounter any issues, we encourage you to
[report them](https://github.com/gittuf/gittuf/issues). This guide assumes you
have already installed gittuf. If you have not, see the [installation guide].

If you prefer to just get up and running with a basic gittuf setup, you can use
the [Express Setup] instructions. Otherwise, if you are familiar with key
management and security policies, see [Manual Setup].

## Express Setup

gittuf includes a built-in setup wizard that will:

 1. Initialize the root of trust in the repository with [Sigstore].
 2. Ask for the GitHub usernames of users authorized to commit to the default
    branch, i.e. `main` or `master`.
 3. Create a rule that authorizes you, and all the users that you supplied in
    Step 2 to make and approve commits to the default branch.

To set up gittuf, run `gittuf setup`, optionally with `--gittuf-token <your
token>` if you wish to have the wizard automatically add GitHub users to the
policy. The rules created by the setup wizard are fully reconfigurable later,
should you wish to change the security policy on the repository.

As this wizard uses [Sigstore] for key management, you will not need to manually
manage signing keys for gittuf's metadata, and may simply login with your
preferred authentication service for [Sigstore] to authenticate instead.

## Manual Setup

gittuf allows you to define the keys and users that are allowed to make changes.
Some familiarity with [The Update Framework] is ideal, as gittuf's metadata is
similar (but not exactly the same) to TUF metadata.

### Create keys

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

### Create a Git repository

gittuf can be used with either a brand-new repository or with an existing
repository. Here, we assume gittuf is being deployed with a fresh repository.
Initialize the repository and gittuf's root of trust metadata using the
key.

```bash
cd .. && mkdir repo && cd repo
git init -q -b main
git config --local gpg.format ssh
git config --local user.signingkey ../keys/developer
```

### Initialize gittuf

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
identity for use with [gitsign]. However, we're using SSH keys throughout in
this guide, as gittuf policy metadata currently cannot be signed using GPG (see
[#229]).

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

### Making repository changes

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

### Verifying policy

gittuf allows for verifying rules for Git references and files.

```sh
gittuf verify-ref --verbose main
```

### Communicating with a remote

gittuf includes two main ways to push and fetch the policy and RSL references.
You may use the `gittuf sync` command to synchronize changes with the remote
automatically. You may also instead use the [gittuf
transport](/internal/git-remote-gittuf), which handles the synchronization of
gittuf metadata transparently upon standard Git pushes and pulls, without
needing to explicitly invoke gittuf.

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
[installation guide]: /docs/installation.md
[Express Setup]: #express-setup
[Manual Setup]: #manual-setup
[The Update Framework]: https://theupdateframework.io/
