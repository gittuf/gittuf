// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package v01

import (
	"encoding/json"
	"fmt"

	gogithub "github.com/google/go-github/v61/github"
	ita "github.com/in-toto/attestation/go/v1"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	PullRequestPredicateType = "https://gittuf.dev/github-pull-request/v0.1"

	digestGitCommitKey = "gitCommit"
)

func NewPullRequestAttestation(owner, repository string, pullRequestNumber int, commitID string, pullRequest *gogithub.PullRequest) (*ita.Statement, error) {
	pullRequestBytes, err := json.Marshal(pullRequest)
	if err != nil {
		return nil, err
	}

	predicate := map[string]any{}
	if err := json.Unmarshal(pullRequestBytes, &predicate); err != nil {
		return nil, err
	}

	predicateStruct, err := structpb.NewStruct(predicate)
	if err != nil {
		return nil, err
	}

	return &ita.Statement{
		Type: ita.StatementTypeUri,
		Subject: []*ita.ResourceDescriptor{
			{
				Uri:    fmt.Sprintf("https://github.com/%s/%s/pull/%d", owner, repository, pullRequestNumber),
				Digest: map[string]string{digestGitCommitKey: commitID},
			},
		},
		PredicateType: PullRequestPredicateType,
		Predicate:     predicateStruct,
	}, nil
}
