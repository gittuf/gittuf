package signers

import (
	"context"
	"encoding/base64"

	"github.com/secure-systems-lab/go-securesystemslib/dsse"
)

func SignEnvelope(envelope *dsse.Envelope, signingKeyContents []byte) (*dsse.Envelope, error) {
	// TODO: pass off to generic LoadSigner
	signer, err := NewEd25519SignerVerifierFromSecureSystemsLibFormat(signingKeyContents)
	if err != nil {
		return &dsse.Envelope{}, err
	}

	keyID, err := signer.KeyID()
	if err != nil {
		return &dsse.Envelope{}, err
	}

	payload, err := base64.StdEncoding.DecodeString(envelope.Payload)
	if err != nil {
		return &dsse.Envelope{}, err
	}

	signature, err := signer.Sign(context.Background(), payload)
	if err != nil {
		return &dsse.Envelope{}, err
	}

	envelope.Signatures = append(envelope.Signatures, dsse.Signature{
		Sig:   base64.StdEncoding.EncodeToString(signature),
		KeyID: keyID,
	})

	return envelope, nil
}

func VerifyEnvelope(envelope *dsse.Envelope, publicKeyContents [][]byte, threshold int) error {
	if threshold < 1 || threshold > len(publicKeyContents) {
		return ErrInvalidThreshold
	}

	verifiers := []dsse.Verifier{}
	for _, k := range publicKeyContents {
		// TODO: pass off to generic LoadVerifier
		v, err := NewEd25519SignerVerifierFromSecureSystemsLibFormat(k)
		if err != nil {
			return err
		}
		verifiers = append(verifiers, dsse.Verifier(*v))
	}

	ev, err := dsse.NewMultiEnvelopeVerifier(threshold, verifiers...)
	if err != nil {
		return err
	}

	_, err = ev.Verify(context.TODO(), envelope)
	return err
}
