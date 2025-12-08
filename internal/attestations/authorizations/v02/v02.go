// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package v02

import (
	"encoding/json"
	"strings"

	"github.com/gittuf/gittuf/internal/attestations/authorizations"
	"github.com/gittuf/gittuf/internal/attestations/common"
	"github.com/gittuf/gittuf/internal/gitinterface"
	sslibdsse "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
	ita "github.com/in-toto/attestation/go/v1"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	PredicateType = "https://gittuf.dev/reference-authorization/v0.2"

	digestGitTreeKey   = "gitTree"
	digestGitCommitKey = "gitCommit"
	targetRefKey       = "targetRef"
	fromIDKey          = "fromID"
	targetIDKey        = "targetID"
)

// ReferenceAuthorization is a lightweight record of a detached authorization in
// a gittuf repository. It is meant to be used as a "predicate" in an in-toto
// attestation.
type ReferenceAuthorization struct {
	TargetRef string `json:"targetRef"`
	FromID    string `json:"fromID"`
	TargetID  string `json:"targetID"`
	Expires   string `json:"expires,omitempty"`
}

func (r *ReferenceAuthorization) GetRef() string {
	return r.TargetRef
}

func (r *ReferenceAuthorization) GetFromID() string {
	return r.FromID
}

func (r *ReferenceAuthorization) GetTargetID() string {
	return r.TargetID
}

func (r *ReferenceAuthorization) GetExpires() string {
	return r.Expires
}

// NewReferenceAuthorizationForCommit creates a new reference authorization for
// the provided information. The authorization is embedded in an in-toto
// "statement" and returned with the appropriate "predicate type" set. The
// `fromID` and `targetID` specify the change to `targetRef` that is to be
// authorized by invoking this function. The targetID is expected to be the Git
// tree ID of the resultant commit.
func NewReferenceAuthorizationForCommit(targetRef, fromID, targetID, expires string) (*ita.Statement, error) {
	predicateStruct, err := newReferenceAuthorizationStruct(targetRef, fromID, targetID, expires)
	if err != nil {
		return nil, err
	}

	return &ita.Statement{
		Type: ita.StatementTypeUri,
		Subject: []*ita.ResourceDescriptor{
			{
				Digest: map[string]string{digestGitTreeKey: targetID},
			},
		},
		PredicateType: PredicateType,
		Predicate:     predicateStruct,
	}, nil
}

// NewReferenceAuthorizationForTag creates a new reference authorization for the
// provided information. The authorization is embedded in an in-toto "statement"
// and returned with the appropriate "predicate type" set. The `fromID` and
// `targetID` specify the change to `targetRef` that is to be authorized by
// invoking this function. The targetID is expected to be the ID of the commit
// the tag will point to.
func NewReferenceAuthorizationForTag(targetRef, fromID, targetID, expires string) (*ita.Statement, error) {
	predicateStruct, err := newReferenceAuthorizationStruct(targetRef, fromID, targetID, expires)
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

// Validate checks that the returned envelope contains the expected in-toto
// attestation and predicate contents.
func Validate(env *sslibdsse.Envelope, targetRef, fromID, targetID string) error {
	payload, err := env.DecodeB64Payload()
	if err != nil {
		return err
	}

	attestation := &ita.Statement{}
	if err := json.Unmarshal(payload, attestation); err != nil {
		return err
	}

	subjectDigest, hasGitTree := attestation.Subject[0].Digest[digestGitTreeKey]
	if hasGitTree {
		if subjectDigest != targetID {
			return authorizations.ErrInvalidAuthorization
		}
	} else {
		subjectDigest, hasGitCommit := attestation.Subject[0].Digest[digestGitCommitKey]
		if !hasGitCommit {
			return authorizations.ErrInvalidAuthorization
		}

		if subjectDigest != targetID {
			return authorizations.ErrInvalidAuthorization
		}

		if !strings.HasPrefix(targetRef, gitinterface.TagRefPrefix) {
			return authorizations.ErrInvalidAuthorization
		}
	}

	predicate := attestation.Predicate.AsMap()

	if predicate[targetIDKey] != targetID {
		return authorizations.ErrInvalidAuthorization
	}

	if predicate[fromIDKey] != fromID {
		return authorizations.ErrInvalidAuthorization
	}

	if predicate[targetRefKey] != targetRef {
		return authorizations.ErrInvalidAuthorization
	}

	return nil
}

func newReferenceAuthorizationStruct(targetRef, fromID, targetID, expires string) (*structpb.Struct, error) {
	predicate := &ReferenceAuthorization{
		TargetRef: targetRef,
		FromID:    fromID,
		TargetID:  targetID,
		Expires:   expires,
	}

	return common.PredicateToPBStruct(predicate)
}
