// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gittuf

// SKETCH EXPERIMENT (cedar-integration-sketch, throwaway, not for merge).
// Exercises the real GitHub pull request attestation creation path
// (AddGitHubPullRequestAttestationForNumber -> addGitHubPullRequestAttestation)
// against a fake GitHub API and reports where the attestation is stored and
// which predicate fields survive the JSON -> structpb round trip.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	githubopts "github.com/gittuf/gittuf/experimental/gittuf/options/github"
	"github.com/gittuf/gittuf/internal/attestations"
	sslibdsse "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
	"github.com/gittuf/gittuf/pkg/gitinterface"
)

func TestCedarSketchPullRequestAttestation(t *testing.T) {
	const mergeSHA = "1111111111111111111111111111111111111111"

	prJSON := func(labels string) string {
		return fmt.Sprintf(`{
  "number": 7,
  "state": "closed",
  "draft": false,
  "merged": true,
  "merged_at": "2026-10-20T10:00:00Z",
  "merge_commit_sha": %q,
  "user": {"login": "jane", "id": 1001},
  "merged_by": {"login": "jane", "id": 1001},
  "labels": %s,
  "requested_reviewers": [],
  "base": {"ref": "main", "sha": "2222222222222222222222222222222222222222", "user": {"login": "octo", "id": 9}},
  "head": {"ref": "feature", "sha": "3333333333333333333333333333333333333333", "user": {"login": "jane", "id": 1001}}
}`, mergeSHA, labels)
	}

	for _, tc := range []struct {
		name   string
		labels string
	}{
		{"no labels", `[]`},
		{"with label", `[{"id": 5, "name": "security-reviewed"}]`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/v3/repos/octo/demo/pulls/7" {
					http.NotFound(w, r)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(prJSON(tc.labels)))
			}))
			defer srv.Close()

			// githubopts.DefaultOptions is a shared pointer that the API
			// mutates; restore it so this experiment doesn't leak options.
			saved := *githubopts.DefaultOptions
			defer func() { *githubopts.DefaultOptions = saved }()

			repo := gitinterface.CreateTestGitRepository(t, t.TempDir(), false)
			r := &Repository{r: newStorer(repo)}
			signer := setupSSHKeysForSigning(t, targetsKeyBytes, targetsPubKeyBytes)

			err := r.AddGitHubPullRequestAttestationForNumber(testCtx, signer, "octo", "demo", 7, false,
				githubopts.WithGitHubBaseURL(srv.URL), githubopts.WithGitHubTokenSource(tokenSourceFunc(func() (string, error) { return "fake", nil })))
			if err != nil {
				t.Fatal(err)
			}

			refTip, err := repo.GetReference(attestations.Ref)
			if err != nil {
				t.Fatal(err)
			}
			treeID, err := repo.GetCommitTreeID(refTip)
			if err != nil {
				t.Fatal(err)
			}
			files, err := repo.GetAllFilesInTree(treeID)
			if err != nil {
				t.Fatal(err)
			}
			paths := []string{}
			for p := range files {
				paths = append(paths, p)
			}
			sort.Strings(paths)
			t.Logf("attestations tree paths: %v", paths)

			var blobPath string
			for _, p := range paths {
				if strings.HasPrefix(p, "github-pull-requests/") {
					blobPath = p
				}
			}
			if blobPath == "" {
				t.Fatal("no PR attestation stored")
			}
			expected := "github-pull-requests/octo-9/refs/heads/main/" + mergeSHA
			t.Logf("stored at %q (expected %q, match=%t)", blobPath, expected, blobPath == expected)

			envBytes, err := repo.ReadBlob(files[blobPath])
			if err != nil {
				t.Fatal(err)
			}
			env := &sslibdsse.Envelope{}
			if err := json.Unmarshal(envBytes, env); err != nil {
				t.Fatal(err)
			}
			keyIDs := []string{}
			for _, s := range env.Signatures {
				keyIDs = append(keyIDs, s.KeyID)
			}
			t.Logf("envelope signature key IDs: %v (signer passed in by caller)", keyIDs)

			payload, err := env.DecodeB64Payload()
			if err != nil {
				t.Fatal(err)
			}
			stmt := map[string]any{}
			if err := json.Unmarshal(payload, &stmt); err != nil {
				t.Fatal(err)
			}
			keys := []string{}
			for k := range stmt {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			t.Logf("statement keys=%v predicate_type=%v subject=%v", keys, stmt["predicate_type"], stmt["subject"])
			pred := stmt["predicate"].(map[string]any)

			get := func(path string) (any, bool) {
				var cur any = pred
				for _, part := range strings.Split(path, ".") {
					m, ok := cur.(map[string]any)
					if !ok {
						return nil, false
					}
					cur, ok = m[part]
					if !ok {
						return nil, false
					}
				}
				return cur, true
			}
			for _, field := range []string{"user.login", "user.id", "labels", "draft", "merged", "merged_by.login", "base.ref", "base.user.login", "head.ref", "head.sha", "merge_commit_sha", "requested_reviewers"} {
				v, ok := get(field)
				t.Logf("  %-20s present=%-5t value=%v (%T)", field, ok, v, v)
			}
		})
	}
}

type tokenSourceFunc func() (string, error)

func (f tokenSourceFunc) Token(_ context.Context) (string, error) { return f() }
