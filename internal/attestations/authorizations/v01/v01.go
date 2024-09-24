// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package v01

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/gittuf/gittuf/internal/attestations/common"
	"github.com/gittuf/gittuf/internal/signerverifier/dsse"
	sslibdsse "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
	ita "github.com/in-toto/attestation/go/v1"
)

const (
	ReferenceAuthorizationPredicateType = "https://gittuf.dev/reference-authorization/v0.1"

	digestGitTreeKey  = "gitTree"
	targetRefKey      = "targetRef"
	fromRevisionIDKey = "fromRevisionID"
	targetTreeIDKey   = "targetTreeID"
)

var ErrInvalidAuthorization = errors.New("authorization attestation does not match expected details")

// ReferenceAuthorization is a lightweight record of a detached authorization in
// a gittuf repository. It is meant to be used as a "predicate" in an in-toto
// attestation.
type ReferenceAuthorization struct {
	TargetRef      string `json:"targetRef"`
	FromRevisionID string `json:"fromRevisionID"`
	TargetTreeID   string `json:"targetTreeID"`
}

func (r *ReferenceAuthorization) GetRef() string {
	return r.TargetRef
}

func (r *ReferenceAuthorization) GetFromID() string {
	return r.FromRevisionID
}

func (r *ReferenceAuthorization) GetTargetID() string {
	return r.TargetTreeID
}

// NewReferenceAuthorization creates a new reference authorization for the
// provided information. The authorization is embedded in an in-toto "statement"
// and returned with the appropriate "predicate type" set. The `fromTargetID`
// and `toTargetID` specify the change to `targetRef` that is to be authorized
// by invoking this function.
func NewReferenceAuthorization(targetRef, fromRevisionID, targetTreeID string) (*ita.Statement, error) {
	predicate := &ReferenceAuthorization{
		TargetRef:      targetRef,
		FromRevisionID: fromRevisionID,
		TargetTreeID:   targetTreeID,
	}

	predicateStruct, err := common.PredicateToPBStruct(predicate)
	if err != nil {
		return nil, err
	}

	return &ita.Statement{
		Type: ita.StatementTypeUri,
		Subject: []*ita.ResourceDescriptor{
			{
				Digest: map[string]string{digestGitTreeKey: targetTreeID},
			},
		},
		PredicateType: ReferenceAuthorizationPredicateType,
		Predicate:     predicateStruct,
	}, nil
}

// Validate checks that the returned envelope contains the expected in-toto
// attestation and predicate contents.
func Validate(env *sslibdsse.Envelope, targetRef, fromRevisionID, targetTreeID string) error {
	payload, err := env.DecodeB64Payload()
	if err != nil {
		return err
	}

	attestation := &ita.Statement{}
	if err := json.Unmarshal(payload, attestation); err != nil {
		return err
	}

	if attestation.Subject[0].Digest[digestGitTreeKey] != targetTreeID {
		return ErrInvalidAuthorization
	}

	predicate := attestation.Predicate.AsMap()

	if predicate[targetTreeIDKey] != targetTreeID {
		return ErrInvalidAuthorization
	}

	if predicate[fromRevisionIDKey] != fromRevisionID {
		return ErrInvalidAuthorization
	}

	if predicate[targetRefKey] != targetRef {
		return ErrInvalidAuthorization
	}

	return nil
}

func CreateTestEnvelope(t *testing.T, refName, fromID, toID string) *sslibdsse.Envelope {
	t.Helper()

	authorization, err := NewReferenceAuthorization(refName, fromID, toID)
	if err != nil {
		t.Fatal(err)
	}
	env, err := dsse.CreateEnvelope(authorization)
	if err != nil {
		t.Fatal(err)
	}

	return env
}
