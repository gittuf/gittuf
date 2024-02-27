// SPDX-License-Identifier: Apache-2.0

package dsse

import (
	"context"
	"encoding/base64"
	"encoding/json"

	"github.com/gittuf/gittuf/internal/signerverifier/common"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
)

const PayloadType = "application/vnd.gittuf+json"

// CreateEnvelope is an opinionated interface to create a DSSE envelope. It
// accepts instances of tuf.RootMetadata, tuf.TargetsMetadata, etc. and marshals
// the input prior to storing it as the envelope's payload.
func CreateEnvelope(v any) (*dsse.Envelope, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	return &dsse.Envelope{
		Signatures:  []dsse.Signature{},
		PayloadType: PayloadType,
		Payload:     base64.StdEncoding.EncodeToString(b),
	}, nil
}

// SignEnvelope is an opinionated API to sign DSSE envelopes. It's opinionated
// because it assumes the payload is Base 64 encoded, which is the expectation
// for gittuf metadata. If one or more signatures from the provided signing key
// already exist, they are all removed in favor of the new signature from that
// key.
func SignEnvelope(ctx context.Context, envelope *dsse.Envelope, signer dsse.Signer) (*dsse.Envelope, error) {
	keyID, err := signer.KeyID()
	if err != nil {
		return nil, err
	}

	payload, err := base64.StdEncoding.DecodeString(envelope.Payload)
	if err != nil {
		return nil, err
	}

	pae := dsse.PAE(envelope.PayloadType, payload)
	sigBytes, err := signer.Sign(ctx, pae)
	if err != nil {
		return nil, err
	}

	signature := dsse.Signature{
		Sig:   base64.StdEncoding.EncodeToString(sigBytes),
		KeyID: keyID,
	}

	// Preserve signatures that aren't from signer
	newSignatures := []dsse.Signature{}
	for _, sig := range envelope.Signatures {
		if sig.KeyID != keyID {
			newSignatures = append(newSignatures, sig)
		}
	}
	// Attach new signature from signer
	newSignatures = append(newSignatures, signature)

	// Replace existing list of signatures with new signatures in envelope
	envelope.Signatures = newSignatures

	return envelope, nil
}

// VerifyEnvelope verifies a DSSE envelope against an expected threshold using
// a slice of verifiers passed into it. Threshold indicates the number of
// providers that must validate the envelope.
func VerifyEnvelope(ctx context.Context, envelope *dsse.Envelope, verifiers []dsse.Verifier, threshold int) error {
	if threshold < 1 || threshold > len(verifiers) {
		return common.ErrInvalidThreshold
	}

	ev, err := dsse.NewMultiEnvelopeVerifier(threshold, verifiers...)
	if err != nil {
		return err
	}

	_, err = ev.Verify(ctx, envelope)
	return err
}
