# Recovering from Verification Errors in gittuf

From time to time, there may be a scenario where gittuf verification fails on
the repository, either due to a malicious change or an issue with insufficient
signatures on a legitimate change.

In order to return the repository to a state where it may pass gittuf
verification again, a recovery procedure must be performed. This procedure may
be performed either automatically (to be added in a future update to gittuf), or
manually, by following the steps detailed below.

This guide assumes at least some familiarity with gittuf's design, with respect
to components such as the Reference State Log (RSL). See the [gittuf Design
Document](/docs/design-document.md) for more information.

Manual recovery of a repository follows a three-step process. This process also
assumes that your signing key is specified in the policy as authorized to make
changes to the reference in question.

## 1. Revoking the Problematic Entry (or Entries)

First, you must identify the oldest RSL entry that causes gittuf verification to
fail. To do this, you must run verification with the `--verbose` flag and then
analyze the log output for the problematic entry.

Take the example of running `gittuf verify-ref --verbose main` on the official
gittuf repository:

```bash
time=2025-09-22T15:10:56.557-04:00 level=DEBUG msg="Verifying signature of Git object with ID '235312e7f06c75d4be0ffb3ff87388f10be9882f'..."
time=2025-09-22T15:10:56.569-04:00 level=DEBUG msg="Proceeding with verification of attestations..."
time=2025-09-22T15:10:56.569-04:00 level=DEBUG msg="Using approvers from code review tool attestations..."
time=2025-09-22T15:10:56.569-04:00 level=DEBUG msg="Violation found: verifying Git namespace policies failed, gittuf policy verification failed"
```

The verification workflow reports that a violation was found for the commit
`235312e7f06c75d4be0ffb3ff87388f10be9882f`.

Now, run `gittuf rsl log`, and search for the appropriate entry:

```bash
entry 1243fdb5d4d24516106747cff336cda5e7dc9a1f

  Ref:    refs/heads/update-roadmap
  Target: f5d70c726c4caa142ba61a63a6eb1dad4e3bd6da
  Number: 463

entry cf2f7e1ad70ef5b1be92bf0e8bd9e4461e36571c

  Ref:    refs/heads/update-roadmap
  Target: 00bb3032e50354e04f567cad157ccb9137d42512
  Number: 462

entry 235312e7f06c75d4be0ffb3ff87388f10be9882f

  Ref:    refs/heads/main
  Target: c4c661618dc96351792ef542fe3cd29b15323d94
  Number: 461

entry e900a5845e92a8a764e4f10eeaeee2a7a00a53c2

  Ref:    refs/gittuf/attestations
  Target: a1028ccedff72017efc47d366a0be5c300216b65
  Number: 460

entry db05a0f1dcb6121e6aceb130d3ca9da1e67cd690

  Ref:    refs/gittuf/attestations
  Target: db598f89434648b12b290f628c4db0f8e1f5156e
  Number: 459

entry cb639d702af5d22246039b3978d58bd1af632bb0

  Ref:    refs/heads/main
  Target: 68b94ea3f289f335c964b0d286df7a00404b4af9
  Number: 458
```

In our case, the entry is numbered 461. To restore the repository to a
known-good state, we must first create a _skip annotation_ that informs gittuf
that the entry should not be processed, and there is another entry that will
restore the repository to a known good state. You must also check for any later
RSL entries for the branch in question, and revoke them as well, by adding their
hashes to the below command.

To create this skip annotation, we run the `gittuf rsl annotate` command, like
so:

```bash
gittuf rsl annotate --skip --local-only --message "GitHub API timing issues" 235312e7f06c75d4be0ffb3ff87388f10be9882f
```

After performing this action, running `gittuf rsl log` will now show the entry
as skipped:

```bash
entry 235312e7f06c75d4be0ffb3ff87388f10be9882f (skipped)

  Ref:    refs/heads/main
  Target: c4c661618dc96351792ef542fe3cd29b15323d94
  Number: 461

    Annotation ID: 194200eb770c8025170dd9bcf2dc89f760da8017
    Skip:          yes
    Number:        474
    Message:
      GitHub API timing issues
```

## 2. Recording the Last Known-Good State

Once the invalid entry or entries have been revoked, the second step is to
record in the RSL an entry for the _last known-good state_ for the reference in
question. This is normally the commit that preceded the invalid one(s). In our
case, the last commit to the `main` branch has a hash of
`68b94ea3f289f335c964b0d286df7a00404b4af9`:

```bash
entry cb639d702af5d22246039b3978d58bd1af632bb0

  Ref:    refs/heads/main
  Target: 68b94ea3f289f335c964b0d286df7a00404b4af9
  Number: 458
```

To record an RSL entry for this entry again, you should rewind the branch in
question until the latest commit on the branch is the one preceding any invalid
ones. In our case, we must rewind the `main` branch by one commit. To do this,
you may use the `update-ref` command from Git:

```bash
git update-ref main `68b94ea3f289f335c964b0d286df7a00404b4af9`:
 # Where the hash is the hash of the last good entry on the branch.
```

Next, you must record the RSL entry with `gittuf rsl record`:

```bash
gittuf rsl record --local-only main
```

Finally, reset the main branch to the latest state with `git update-ref`:

```
git update-ref main @{upstream}
```

## 3. Recording the Latest State

Once the previous-good commit has been recorded again in the RSL, you must
record the latest state of the branch, including any fixes on the branch. **This
RSL entry must be signed by a user whose signatures count towards the threshold
required in gittuf policy for the branch in question.** For example, if Alice
and Bob are both required to sign changes that appear on the main branch, either
Alice or Bob must sign this RSL entry.

To do this, record the changes again with `gittuf rsl record`. In our case, this
will be the same command again as above:

```bash
gittuf rsl record --local-only main
```

Make sure to run `gittuf sync` after the process is complete to push the fixes
up to the remote:

```
gittuf sync
```
