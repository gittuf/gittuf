# Inspecting gittuf Metadata

When making changes to gittuf or diagnosing issues with it, there may be a need
to inspect the raw metadata stored in JSON.

## Policy

gittuf policy is stored in the `refs/gittuf/policy` reference. To learn more
about its structure and how various extensions are implemented, see the various
GAPs, available under `/docs/gaps/`.

This file uses the metadata of the `gittuf/gittuf` repository as an example, the
values for other repositories will be different. To view the metadata for
gittuf's policy, you may use the following one-liner:

`git show refs/gittuf/policy:metadata/<file name>.json | jq -r .payload | base64 -d | jq .`

Below is an explanation of how the process works.

1. Obtain the commit hash for the policy state that you'd like to inspect
   metadata for:

```bash
git log refs/gittuf/policy
```

The output will look something like:

```
commit 34360d799489d607ac5d29c8953c5fec9f0ac18f
Author: Aditya Sirish <aditya@saky.in>
Date:   Wed Apr 23 12:39:29 2025 -0400

    Add rule 'protect-releases' to policy 'targets'

commit 5485d2478dbd42a081e7203a75cc21b272515837
Author: Aditya Sirish <aditya@saky.in>
Date:   Wed Apr 23 12:39:18 2025 -0400

    Add rule 'protect-main' to policy 'targets'
```

In this case, we'll inspect the latest policy state, which is commit
`34360d799489d607ac5d29c8953c5fec9f0ac18f`.

2. View the tree information for the specified commit:

```bash
git ls-tree -r 34360d799489d607ac5d29c8953c5fec9f0ac18f
```

The output will look something like:

```
100644 blob 8fd789f032be4dd64990de85ae860a1a6b71bc01	metadata/root.json
100644 blob 67f72e322aebbf4ee95b6694ecdc14ac355a58da	metadata/targets.json
```

Notice the two files that are present: `metadata/root.json` and
`metadata/targets.json`. The process for inspecting metadata diverges here
depending on which set you would like to inspect.

3. Use `git cat-file`, `jq`. and `base64` with the hash of the file
   corresponding to the metadata you'd like to inspect:

```bash
git cat-file -p <hash> | jq -r .payload | base64 -d | jq -r .
```

For example, if you'd like to inspect the root metadata from the example above, the command would be:

```bash
git cat-file -p 8fd789f032be4dd64990de85ae860a1a6b71bc01 | jq -r .payload | base64 -d | jq -r .
```

The command above:
1. Fetches the blob containing the metdataa file you'd like to inspect.
2. Pipes it into `jq` to extract the metadata from the DSSE envelope (i.e. JSON)
   containing the metadata payload and signature.
3. Decodes the base64-encoded metadata to ASCII.
4. Formats the JSON output nicely for the user.

The output will look something like:

```
{
  "type": "root",
  "schemaVersion": "https://gittuf.dev/policy/root/v0.2",
  "expires": "2026-04-23T12:33:38-04:00",
  "principals": {
    "SHA256:KTrCAHHGUSCkNjanR0t4ojOiHQ4qZIQM6mkwX64b2KY": {
      "keyid_hash_algorithms": null,
      "keytype": "ssh",
      "keyval": {
        "public": "AAAAB3NzaC1yc2EAAAADAQABAAABAQCro6oX+Mm6ze34i2NQg8ESVo/34bCh5F3q5ZNhy3i652B5qLddm4Opao4le6bnlFj6zgrHUqOc4sxhpKJNUOAeaxuwa7wuOWOTEPSvY8lJWSZYEgfM7zT/AxOlWOcJRUfBOwyrrRW2Lvh4p8tJ5oSvDAIN/99qNDlr353eBY7AaREu9BbVQHrwyTrC1eZ7ZfYUtaWkChaYOaaHg9ZCINVDeTdjXzT4KD2m3FBAhD1ZlCIQZ6C35kWzdby4re/paUDZIHl8xH/tZ/xNB/+jWyWNpIEhfMS5MH7FN5WNqHZ8mnOi4L+7nDHEvJU2lXt+oS9btctmwVGFkyUMVGHGv+Gr"
      },
      "scheme": "ssh-rsa",
      "keyid": "SHA256:KTrCAHHGUSCkNjanR0t4ojOiHQ4qZIQM6mkwX64b2KY"
    },
    "aditya@saky.in::https://github.com/login/oauth": {
      "keyid_hash_algorithms": null,
      "keytype": "sigstore-oidc",
      "keyval": {
        "identity": "aditya@saky.in",
        "issuer": "https://github.com/login/oauth"
      },
      "scheme": "fulcio",
      "keyid": "aditya@saky.in::https://github.com/login/oauth"
    },
    "billy@chainguard.dev::https://accounts.google.com": {
      "keyid_hash_algorithms": null,
      "keytype": "sigstore-oidc",
      "keyval": {
        "identity": "billy@chainguard.dev",
        "issuer": "https://accounts.google.com"
      },
      "scheme": "fulcio",
      "keyid": "billy@chainguard.dev::https://accounts.google.com"
    }
  },
  "roles": {
    "root": {
      "principalIDs": [
        "aditya@saky.in::https://github.com/login/oauth",
        "billy@chainguard.dev::https://accounts.google.com"
      ],
      "threshold": 1
    },
    "targets": {
      "principalIDs": [
        "aditya@saky.in::https://github.com/login/oauth",
        "billy@chainguard.dev::https://accounts.google.com"
      ],
      "threshold": 1
    }
  },
  "githubApps": {
    "https://gittuf.dev/github-app": {
      "trusted": true,
      "principalIDs": [
        "SHA256:KTrCAHHGUSCkNjanR0t4ojOiHQ4qZIQM6mkwX64b2KY"
      ],
      "threshold": 1
    }
  }
}
```

## Attestations

Attestations created by and used by gittuf are stored in the
`refs/gittuf/attestation` reference. As with the policy example above, see
gittuf's GAPs for details on attestations' structure. The following example is
also taken from the `gittuf/gittuf` repository.

To view attestations in the repository, use the following process:

1. Obtain the commit hash for the attestations state that you'd like to inspect
   metadata for:

```bash
git log refs/gittuf/attestations
```

The output will look something like:

```
commit 75b1c36e2574b53557f84755e3944ec76a89d155
Author: gittuf-github-app <179610826+gittuf-app-test[bot]@users.noreply.github.com>
Date:   Mon Oct 6 18:49:27 2025 +0000

    Add GitHub pull request attestation for 'gittuf-127987161/refs/heads/main' at '71a0daaec1e998931f2ed8c0f613850d224b681f'
    
    Source: https://github.com/gittuf/gittuf/pull/1109

commit c11e46cbba7a18246e3f0fe08822e54dcea19bdb
Author: gittuf-github-app <179610826+gittuf-app-test[bot]@users.noreply.github.com>
Date:   Mon Oct 6 18:48:33 2025 +0000

    Add GitHub pull request approval for 'refs/heads/main' from 'c3ef4dc745e37451446f8b6e4c90b4c17239b973' to '32af1b10212ee9d02b1d49b7b739a2a3ab44d620' (review ID 3306609844) for approval by 'adityasaky+8928778'
```

In this case, we'd like to inspect the attestation capturing the approval of
the pull request, which is added by commit
`c11e46cbba7a18246e3f0fe08822e54dcea19bdb`.

2. View the tree information for the specified commit:

```bash
git ls-tree c11e46cbba7a18246e3f0fe08822e54dcea19bdb
```

The output will look something like:

```
040000 tree cb29826045d64737576029e3b18a703b68877077	code-review-approvals
040000 tree 5db0e6c46a77ddb5589cab7506ff460b4deb6f64	github-pull-requests
040000 tree e99bfd730ea54b765e79c23262a89b77a44f2c4c	reference-authorizations
```

3. Since we want to inspect an attestation that encodes an approval, we must
   further inspect the `code-review-approvals` tree. This time, we can inspect
   recursively as we've limited the attestations that will be shown:

```bash
git ls-tree -r cb29826045d64737576029e3b18a703b68877077
```

Doing so gives us the following output (truncated to show the part of interest):

```
100644 blob 3c40d5a249f4f1c3f5476a5a3e7fe05f6a8e3a67	refs/heads/main/bedce5122c630db87d7036f0b25650f3f8ae438c-4bdc2cfc0f67d9afea2e7271e1d70e0509069ed6/github/aHR0cHM6Ly9naXR0dWYuZGV2L2dpdGh1Yi1hcHA=
100644 blob 431ac2ba408d31d9afa6205c256b4397a58ce0ab	refs/heads/main/c3ef4dc745e37451446f8b6e4c90b4c17239b973-32af1b10212ee9d02b1d49b7b739a2a3ab44d620/github/aHR0cHM6Ly9naXR0dWYuZGV2L2dpdGh1Yi1hcHA=
100644 blob b4806c15328f38fc976138da5b4bf61edab8541e	refs/heads/main/c3ef4dc745e37451446f8b6e4c90b4c17239b973-848a8ce805f207d1e3f1e3ec2c60bb18042aa6a7/github/aHR0cHM6Ly9naXR0dWYuZGV2L2dpdGh1Yi1hcHA=
100644 blob 6f08f80ff34760963456ff5fdb23dcb341586da6	refs/heads/main/c4a5642991ca35a50c3c8d7994aadc8f520abdd1-245a9a636f82348fe31eb2f61e974c2f43d5c333/github/aHR0cHM6Ly9naXR0dWYuZGV2L2dpdGh1Yi1hcHA=
100644 blob 95248cec12070ee96fb099c36938f6d92429be10	refs/heads/main/c4c661618dc96351792ef542fe3cd29b15323d94-041ccb54ec82dc7ae0108632cecb073f11cea3a0/github/aHR0cHM6Ly9naXR0dWYuZGV2L2dpdGh1Yi1hcHA=

```

4. Finding the attestation we'd like to inspect is a bit more complicated than
   the policy, but we can find it by reading the message for the commit which added
   the attestation. Of interest are the commit hashes in the line:
```
Add GitHub pull request approval for 'refs/heads/main' from 'c3ef4dc745e37451446f8b6e4c90b4c17239b973' to '32af1b10212ee9d02b1d49b7b739a2a3ab44d620' (review ID 3306609844) for approval by 'adityasaky+8928778'
```

Searching through the attestations listed before yields the correct blob:
```
100644 blob 431ac2ba408d31d9afa6205c256b4397a58ce0ab	refs/heads/main/c3ef4dc745e37451446f8b6e4c90b4c17239b973-32af1b10212ee9d02b1d49b7b739a2a3ab44d620/github/aHR0cHM6Ly9naXR0dWYuZGV2L2dpdGh1Yi1hcHA=
```

5. Finally, to extract and decode the attestation, we use the same method as
   with the policy:

```bash
git cat-file -p 431ac2ba408d31d9afa6205c256b4397a58ce0ab | jq -r .payload | base64 -d | jq -r .
```

The output will look something like:

```
{
  "type": "https://in-toto.io/Statement/v1",
  "subject": [
    {
      "digest": {
        "gitTree": "32af1b10212ee9d02b1d49b7b739a2a3ab44d620"
      }
    }
  ],
  "predicate_type": "https://gittuf.dev/github-pull-request-approval/v0.1",
  "predicate": {
    "approvers": [
      "adityasaky+8928778"
    ],
    "dismissedApprovers": [],
    "fromRevisionID": "c3ef4dc745e37451446f8b6e4c90b4c17239b973",
    "targetRef": "refs/heads/main",
    "targetTreeID": "32af1b10212ee9d02b1d49b7b739a2a3ab44d620"
  }
}
```
