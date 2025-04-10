// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package v01

import (
	"github.com/gittuf/gittuf/internal/attestations/common"
	"github.com/gittuf/gittuf/internal/tuf"
	ita "github.com/in-toto/attestation/go/v1"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	PredicateType = "https://gittuf.dev/hook-execution/v0.1"

	digestGitTreeKey = "gitTree"
)

type HookExecutionAttestation struct {
	TargetRef   string   `json:"targetRef"`
	TargetID    string   `json:"targetID"`
	PolicyEntry string   `json:"policyEntry"`
	Stage       string   `json:"hookStage"`
	HookNames   []string `json:"hooks"`
	Runner      string   `json:"principalID"`
}

func (ha *HookExecutionAttestation) GetTargetRef() string {
	return ha.TargetRef
}

func (ha *HookExecutionAttestation) GetPolicyEntry() string {
	return ha.PolicyEntry
}

func (ha *HookExecutionAttestation) GetHookStage() string {
	return ha.Stage
}

func (ha *HookExecutionAttestation) GetHooks() []string {
	return ha.HookNames
}

func (ha *HookExecutionAttestation) GetHookRunner() string {
	return ha.Runner
}

// NewHookExecutionAttestationForStage creates a new hook execution attestation
// for the provided information. The attestation is embedded in an in in-toto
// "statement" and returned with the appropriate "predicate type" set. The
// `targetRef` specifies which branch was currently in use when running the
// hooks, while the `policyEntry` specifies which policy entry the hooks were
// loaded from.
func NewHookExecutionAttestationForStage(targetRef, targetID, policyEntry string, stage tuf.HookStage, hookNames []string, executor string) (*ita.Statement, error) {
	predicateStruct, err := newHookExecutionAttestationStruct(targetRef, targetID, policyEntry, stage, hookNames, executor)
	if err != nil {
		return nil, err
	}

	return &ita.Statement{
		Type: ita.StatementTypeUri,
		Subject: []*ita.ResourceDescriptor{
			{
				Digest: map[string]string{digestGitTreeKey: targetRef},
			},
		},
		PredicateType: PredicateType,
		Predicate:     predicateStruct,
	}, nil
}

func newHookExecutionAttestationStruct(targetRef, targetID, policyEntry string, stage tuf.HookStage, hookNames []string, executor string) (*structpb.Struct, error) {
	predicate := &HookExecutionAttestation{
		TargetRef:   targetRef,
		TargetID:    targetID,
		PolicyEntry: policyEntry,
		Stage:       stage.String(),
		HookNames:   hookNames,
		Runner:      executor,
	}
	return common.PredicateToPBStruct(predicate)
}
