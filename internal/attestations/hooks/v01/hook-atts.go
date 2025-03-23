package v01

import (
	"github.com/gittuf/gittuf/internal/attestations/common"
	"github.com/gittuf/gittuf/internal/gitinterface"
	ita "github.com/in-toto/attestation/go/v1"
)

const (
	digestGitTreeKey                  = "gitTree"
	HooksExecutionReportPredicateType = "https://gittuf.dev/hook-execution-report/v0.1"
)

// what will the HookAttestation structure contain?

// author
// authorized changers
// hash of the commit?
// name of the hooks?

// type HooksExecutionReport struct {
//	StageAttestationReferences                map[string]gitinterface.Hash `json:"stage_attestation_references"` // reference the attestation for that stage
//	HooksExecutor                             string                       `json:"hooks_executor"`               // should this be a key_id?
//	*authorizationsv01.ReferenceAuthorization                              // do we need this?
// }

type HooksAttestationForStage struct {
	Stage               string                       `json:"stage"`
	HookNameCommitIDMap map[string]gitinterface.Hash `json:"hook_name_commit_id_map"` // {"hook1.py": S0M3H4SH, "hook2.py": S0M30TH3RH4SH}
	Executor            string
}

// func (h *HooksExecutionReport) ReturnHooksExecutionStatuses() map[string]gitinterface.Hash {
//	return h.StageAttestationReferences
// }

func NewHooksAttestationForStage(stage, executor string, hookNameCommitIDMap map[string]gitinterface.Hash) (*ita.Statement, error) {
	predicate := &HooksAttestationForStage{
		Stage:               stage,
		HookNameCommitIDMap: hookNameCommitIDMap,
		Executor:            executor,
	}

	predicateStruct, err := common.PredicateToPBStruct(predicate)
	if err != nil {
		return nil, err
	}

	// unsure about what to return in this body
	return &ita.Statement{
		Type: ita.StatementTypeUri,
		Subject: []*ita.ResourceDescriptor{
			{
				// Digest: map[string]string{hookname: some_identifier // avoid this for now.
			},
		}, // set of software artifacts that the attestation applies to.
		PredicateType: HooksExecutionReportPredicateType,
		Predicate:     predicateStruct,
	}, nil
}

// NewHooksAttestationReport will create and return a new HooksExecutionReport object.
// Arguments should be hooks execution status for each type of hook and some
// identifier for the id which executed the hook
// func NewHooksAttestationReport(executor, targetTreeID string, hooksExecutionStatuses map[string]gitinterface.Hash) (*ita.Statement, error) {
//
//	predicate := &HooksExecutionReport{
//		StageAttestationReferences: hooksExecutionStatuses,
//		HooksExecutor:              executor,
//	}
//
//	predicateStruct, err := common.PredicateToPBStruct(predicate)
//	if err != nil {
//		return nil, err
//	}
//
//	// unsure about what to return in this body
//	return &ita.Statement{
//		Type: ita.StatementTypeUri,
//		Subject: []*ita.ResourceDescriptor{
//			{
//				Digest: map[string]string{digestGitTreeKey: targetTreeID},
//			},
//		},
//		PredicateType: HooksExecutionReportPredicateType, // what to do about this?
//		Predicate:     predicateStruct,
//	}, nil
// }
