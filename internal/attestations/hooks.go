package attestations

import (
	"encoding/json"

	hooksv01 "github.com/gittuf/gittuf/internal/attestations/hooks/v01"
	"github.com/gittuf/gittuf/internal/gitinterface"
	sslibdsse "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
	ita "github.com/in-toto/attestation/go/v1"
)

func NewHooksAttestation(stage, executor string, hookNameCommitIDMap map[string]gitinterface.Hash) (*ita.Statement, error) {
	return hooksv01.NewHooksAttestationForStage(stage, executor, hookNameCommitIDMap)
}

func (a *Attestations) SetHookAttestation(repo *gitinterface.Repository, env *sslibdsse.Envelope, stage string) error {
	envBytes, err := json.Marshal(env)
	if err != nil {
		return err
	}

	blobID, err := repo.WriteBlob(envBytes)
	if err != nil {
		return err
	}

	if a.hooksAttestations == nil {
		a.hooksAttestations = map[string]gitinterface.Hash{}
	}
	a.hooksAttestations[stage] = blobID

	return nil
}
