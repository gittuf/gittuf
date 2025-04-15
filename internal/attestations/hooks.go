// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package attestations

import (
	"encoding/json"

	hooksv01 "github.com/gittuf/gittuf/internal/attestations/hooks/v01"
	"github.com/gittuf/gittuf/internal/gitinterface"
	sslibdsse "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
	"github.com/gittuf/gittuf/internal/tuf"
	ita "github.com/in-toto/attestation/go/v1"
)

// NewHookExecutionAttestation creates a new
func NewHookExecutionAttestation(targetRef, targetID, policyEntry string, stage tuf.HookStage, hookNames []string, executor string) (*ita.Statement, error) {
	return hooksv01.NewHookExecutionAttestationForStage(targetRef, targetID, policyEntry, stage, hookNames, executor)
}

func (a *Attestations) SetHookExecutionAttestation(repo *gitinterface.Repository, env *sslibdsse.Envelope, stage string) error {
	envBytes, err := json.Marshal(env)
	if err != nil {
		return err
	}

	blobID, err := repo.WriteBlob(envBytes)
	if err != nil {
		return err
	}

	if a.hookExecutionAttestations == nil {
		a.hookExecutionAttestations = map[string]gitinterface.Hash{}
	}
	a.hookExecutionAttestations[stage] = blobID

	return nil
}
