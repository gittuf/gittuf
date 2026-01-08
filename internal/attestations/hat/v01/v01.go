// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package v01

import (
	"github.com/gittuf/gittuf/internal/attestations/common"
	sslibdsse "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
	ita "github.com/in-toto/attestation/go/v1"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	PredicateType = "https://gittuf.dev/hat-attestation/v0.1"

	digestGitTreeKey   = "gitTree"
	digestGitCommitKey = "gitCommit"
)

type HatAttestation struct {
	TargetRef string `json:"targetRef"`
	TargetID  string `json:"targetID"`
	TeamID    string `json:"teamID"`
}

func (ha *HatAttestation) GetRef() string {
	return ha.TargetRef
}

func (ha *HatAttestation) GetTargetID() string {
	return ha.TargetID
}

func (ha *HatAttestation) GetTeamID() string {
	return ha.TeamID
}

func NewHatAttestationForCommit(targetRef, targetID, teamID string) (*ita.Statement, error) {
	predicateStruct, err := newHatAttestationStruct(targetRef, targetID, teamID)
	if err != nil {
		return nil, err
	}

	return &ita.Statement{
		Type: ita.StatementTypeUri,
		Subject: []*ita.ResourceDescriptor{
			{
				Digest: map[string]string{digestGitCommitKey: targetID},
			},
		},
		PredicateType: PredicateType,
		Predicate:     predicateStruct,
	}, nil
}

func NewHatAttestationForTag(targetRef, targetID, teamID string) (*ita.Statement, error) {
	predicateStruct, err := newHatAttestationStruct(targetRef, targetID, teamID)
	if err != nil {
		return nil, err
	}

	return &ita.Statement{
		Type: ita.StatementTypeUri,
		Subject: []*ita.ResourceDescriptor{
			{
				Digest: map[string]string{digestGitCommitKey: targetID},
			},
		},
		PredicateType: PredicateType,
		Predicate:     predicateStruct,
	}, nil
}

// TODO
func Validate(env *sslibdsse.Envelope, targetRef, targetID, teamID string) error {
	return nil
}

func newHatAttestationStruct(targetRef, targetID, teamID string) (*structpb.Struct, error) {
	predicate := &HatAttestation{
		TargetRef: targetRef,
		TargetID:  targetID,
		TeamID:    teamID,
	}

	return common.PredicateToPBStruct(predicate)
}
