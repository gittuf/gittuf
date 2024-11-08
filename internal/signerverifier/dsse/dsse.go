// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package dsse

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/gittuf/gittuf/internal/signerverifier/common"
	"github.com/gittuf/gittuf/internal/signerverifier/sigstore"
	"github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
	protobundle "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
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

	var signature dsse.Signature
	if _, isSigstoreSigner := signer.(*sigstore.Signer); isSigstoreSigner {
		// Unpack the bundle to get the signature + verification material
		// Set extension in the signature object

		bundle := protobundle.Bundle{}
		if err := protojson.Unmarshal(sigBytes, &bundle); err != nil {
			return nil, err
		}

		actualSigBytes, err := protojson.Marshal(bundle.GetMessageSignature())
		if err != nil {
			return nil, err
		}

		verificationMaterial := bundle.GetVerificationMaterial()
		verificationMaterialBytes, err := protojson.Marshal(verificationMaterial)
		if err != nil {
			return nil, err
		}
		verificationMaterialStruct := new(structpb.Struct)
		if err := protojson.Unmarshal(verificationMaterialBytes, verificationMaterialStruct); err != nil {
			return nil, err
		}

		signature = dsse.Signature{
			Sig:   base64.StdEncoding.EncodeToString(actualSigBytes),
			KeyID: keyID,
			Extension: &dsse.Extension{
				Kind: sigstore.ExtensionMimeType,
				Ext:  verificationMaterialStruct,
			},
		}
	} else {
		signature = dsse.Signature{
			Sig:   base64.StdEncoding.EncodeToString(sigBytes),
			KeyID: keyID,
		}
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
func VerifyEnvelope(ctx context.Context, envelope *dsse.Envelope, verifiers []dsse.Verifier, threshold int) ([]dsse.AcceptedKey, error) {
	if threshold < 1 {
		return nil, common.ErrInvalidThreshold
	}

	ev, err := dsse.NewEnvelopeVerifier(verifiers...)
	if err != nil {
		return nil, err
	}

	// We verify with threshold == 1 because we want control over the threshold
	// checks: we get all the verified keys back
	acceptedKeys, err := ev.Verify(ctx, envelope)
	if err != nil {
		return nil, err
	}

	if len(acceptedKeys) < threshold {
		return acceptedKeys, fmt.Errorf("accepted signatures do not match threshold, Found: %d, Expected %d", len(acceptedKeys), threshold)
	}
	return acceptedKeys, nil
}
