package attestations

import (
	"context"
	"encoding/json"
	"time"

	dsse "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
)

// TemporaryApprovalAttestation defines the structure for time-limited approvals.
type TemporaryApprovalAttestation struct {
	ActionID       string    `json:"action_id"`
	ApproverKeyID  string    `json:"approver_key_id"`
	ApprovalTime   time.Time `json:"approval_time"`
	ExpirationTime time.Time `json:"expiration_time"`
}

// Media type identifier for the payload
const TemporaryApprovalMediaType = "application/vnd.gittuf.temp-approval+json"

// SignTemporaryApproval creates a DSSE envelope over the JSON-serialized attestation
func SignTemporaryApproval(ctx context.Context, att *TemporaryApprovalAttestation, signer dsse.Signer) (*dsse.Envelope, error) {
	payload, err := json.Marshal(att)
	if err != nil {
		return nil, err
	}

	envSigner, err := dsse.NewEnvelopeSigner(signer)
	if err != nil {
		return nil, err
	}

	envelope, err := envSigner.SignPayload(ctx, TemporaryApprovalMediaType, payload)
	if err != nil {
		return nil, err
	}

	return envelope, nil
}
