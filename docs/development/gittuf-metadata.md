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
