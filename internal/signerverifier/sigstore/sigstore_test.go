// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package sigstore

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/cyberphone/json-canonicalization/go/src/webpki.org/jsoncanonicalizer"
	signeropts "github.com/gittuf/gittuf/internal/signerverifier/sigstore/options/signer"
	verifieropts "github.com/gittuf/gittuf/internal/signerverifier/sigstore/options/verifier"
	protobundle "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
	protocommon "github.com/sigstore/protobuf-specs/gen/pb-go/common/v1"
	protorekor "github.com/sigstore/protobuf-specs/gen/pb-go/rekor/v1"
	"github.com/sigstore/rekor/pkg/pki"
	"github.com/sigstore/rekor/pkg/types"
	"github.com/sigstore/rekor/pkg/types/hashedrekord"
	rekorutil "github.com/sigstore/rekor/pkg/util"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/sign"
	testca "github.com/sigstore/sigstore-go/pkg/testing/ca"
	"github.com/sigstore/sigstore-go/pkg/tlog"
	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	testIdentity = "sigstore-test@example.com"
	testIssuer   = "https://issuer.example.test"
	testIDToken  = "test-id-token"
)

type localTrustedRoot struct {
	root.BaseTrustedMaterial
	fulcioCA *root.FulcioCertificateAuthority
	rekorID  string
	rekorLog *root.TransparencyLog
}

func (r *localTrustedRoot) FulcioCertificateAuthorities() []root.CertificateAuthority {
	return []root.CertificateAuthority{r.fulcioCA}
}

func (r *localTrustedRoot) RekorLogs() map[string]*root.TransparencyLog {
	return map[string]*root.TransparencyLog{r.rekorID: r.rekorLog}
}

type localFulcio struct {
	identity        string
	issuer          string
	intermediate    *x509.Certificate
	intermediateKey *ecdsa.PrivateKey
	called          bool
	providedIDToken string
}

func (f *localFulcio) GetCertificate(_ context.Context, keypair sign.Keypair, options *sign.CertificateProviderOptions) ([]byte, error) {
	f.called = true
	if options != nil {
		f.providedIDToken = options.IDToken
	}

	now := time.Now()
	leafTemplate := &x509.Certificate{
		SerialNumber:   big.NewInt(1),
		NotBefore:      now.Add(-time.Minute),
		NotAfter:       now.Add(time.Hour),
		EmailAddresses: []string{f.identity},
		KeyUsage:       x509.KeyUsageDigitalSignature,
		ExtKeyUsage:    []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning},
		ExtraExtensions: []pkix.Extension{{
			Id:       asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 1},
			Value:    []byte(f.issuer),
			Critical: false,
		}},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTemplate, f.intermediate, keypair.GetPublicKey(), f.intermediateKey)
	if err != nil {
		return nil, err
	}
	return leafDER, nil
}

type localRekor struct {
	privateKey *ecdsa.PrivateKey
	logID      string
	logIDBytes []byte
	called     bool
}

func (r *localRekor) GetTransparencyLogEntry(ctx context.Context, publicKeyOrCertificate []byte, bundle *protobundle.Bundle) error {
	r.called = true
	messageSignature := bundle.GetMessageSignature()
	if messageSignature == nil {
		return errors.New("Sigstore bundle has no message signature")
	}

	artifactProperties := types.ArtifactProperties{
		PublicKeyBytes: [][]byte{publicKeyOrCertificate},
		PKIFormat:      string(pki.X509),
		SignatureBytes: messageSignature.Signature,
		ArtifactHash:   rekorutil.PrefixSHA(hex.EncodeToString(messageSignature.MessageDigest.Digest)),
	}
	proposedEntry, err := hashedrekord.New().CreateProposedEntry(ctx, "", artifactProperties)
	if err != nil {
		return err
	}
	versionedEntry, err := types.CreateVersionedEntry(proposedEntry)
	if err != nil {
		return err
	}
	canonicalizedBody, err := types.CanonicalizeEntry(ctx, versionedEntry)
	if err != nil {
		return err
	}

	integratedTime := time.Now().Unix()
	payload, err := json.Marshal(tlog.RekorPayload{
		Body:           base64.StdEncoding.EncodeToString(canonicalizedBody),
		IntegratedTime: integratedTime,
		LogIndex:       0,
		LogID:          r.logID,
	})
	if err != nil {
		return err
	}
	canonicalizedPayload, err := jsoncanonicalizer.Transform(payload)
	if err != nil {
		return err
	}
	logSigner, err := signature.LoadECDSASignerVerifier(r.privateKey, crypto.SHA256)
	if err != nil {
		return err
	}
	signedEntryTimestamp, err := logSigner.SignMessage(bytes.NewReader(canonicalizedPayload))
	if err != nil {
		return err
	}
	leafHash := sha256.Sum256(append([]byte{0}, canonicalizedBody...))
	checkpoint, err := rekorutil.CreateAndSignCheckpoint(ctx, "rekor.localhost", 0, 1, leafHash[:], logSigner)
	if err != nil {
		return err
	}
	if bundle.VerificationMaterial == nil {
		bundle.VerificationMaterial = &protobundle.VerificationMaterial{}
	}
	bundle.VerificationMaterial.TlogEntries = append(
		bundle.VerificationMaterial.TlogEntries,
		&protorekor.TransparencyLogEntry{
			LogIndex:          0,
			LogId:             &protocommon.LogId{KeyId: r.logIDBytes},
			IntegratedTime:    integratedTime,
			KindVersion:       &protorekor.KindVersion{Kind: "hashedrekord", Version: "0.0.1"},
			CanonicalizedBody: canonicalizedBody,
			InclusionPromise: &protorekor.InclusionPromise{
				SignedEntryTimestamp: signedEntryTimestamp,
			},
			InclusionProof: &protorekor.InclusionProof{
				LogIndex:   0,
				RootHash:   leafHash[:],
				TreeSize:   1,
				Checkpoint: &protorekor.Checkpoint{Envelope: string(checkpoint)},
			},
		},
	)
	return nil
}

func newLocalSigstore(t *testing.T) (*localTrustedRoot, *localFulcio, *localRekor) {
	t.Helper()

	rootCertificate, rootKey, err := testca.GenerateRootCa()
	require.NoError(t, err)
	intermediate, intermediateKey, err := testca.GenerateFulcioIntermediate(rootCertificate, rootKey)
	require.NoError(t, err)
	rekorKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	publicKeyDER, err := x509.MarshalPKIXPublicKey(rekorKey.Public())
	require.NoError(t, err)
	logIDHash := sha256.Sum256(publicKeyDER)
	logID := hex.EncodeToString(logIDHash[:])
	logIDBytes, err := hex.DecodeString(logID)
	require.NoError(t, err)

	now := time.Now()
	fulcioCA := &root.FulcioCertificateAuthority{
		URI:                 testIssuer,
		Root:                rootCertificate,
		Intermediates:       []*x509.Certificate{intermediate},
		ValidityPeriodStart: now.Add(-time.Hour),
		ValidityPeriodEnd:   now.Add(time.Hour),
	}
	trustedRoot := &localTrustedRoot{
		fulcioCA: fulcioCA,
		rekorID:  logID,
		rekorLog: &root.TransparencyLog{
			BaseURL:             "local-test-rekor",
			ID:                  []byte(logID),
			ValidityPeriodStart: now.Add(-time.Hour),
			ValidityPeriodEnd:   now.Add(time.Hour),
			HashFunc:            crypto.SHA256,
			PublicKey:           rekorKey.Public(),
			SignatureHashFunc:   crypto.SHA256,
		},
	}
	fulcio := &localFulcio{
		identity:        testIdentity,
		issuer:          testIssuer,
		intermediate:    intermediate,
		intermediateKey: intermediateKey,
	}
	rekor := &localRekor{
		privateKey: rekorKey,
		logID:      logID,
		logIDBytes: logIDBytes,
	}

	return trustedRoot, fulcio, rekor
}

func TestSignerSignAndVerifierVerify(t *testing.T) {
	trustedRoot, fulcio, rekor := newLocalSigstore(t)
	data := []byte("gittuf Sigstore test payload")

	signer := NewSigner(
		signeropts.WithFulcio(fulcio),
		signeropts.WithRekor(rekor),
	)
	signer.token = testIDToken
	signer.identity = testIdentity
	signer.issuer = testIssuer

	bundleBytes, err := signer.Sign(t.Context(), data)
	require.NoError(t, err)
	require.True(t, fulcio.called, "signing must request a certificate from the injected Fulcio")
	require.Equal(t, testIDToken, fulcio.providedIDToken)
	require.True(t, rekor.called, "signing must submit to the injected Rekor")

	signedBundle := new(protobundle.Bundle)
	require.NoError(t, protojson.Unmarshal(bundleBytes, signedBundle))
	require.NotNil(t, signedBundle.GetMessageSignature())
	require.NotEmpty(t, signedBundle.GetVerificationMaterial().GetTlogEntries())

	verificationMaterialJSON, err := protojson.Marshal(signedBundle.GetVerificationMaterial())
	require.NoError(t, err)
	verificationMaterial := new(structpb.Struct)
	require.NoError(t, protojson.Unmarshal(verificationMaterialJSON, verificationMaterial))
	signatureJSON, err := protojson.Marshal(signedBundle.GetMessageSignature())
	require.NoError(t, err)

	verifier := NewVerifierFromIdentityAndIssuer(
		testIdentity,
		testIssuer,
		verifieropts.WithTrustedRoot(trustedRoot),
	)
	verifier.SetExtension(verificationMaterial)
	require.NoError(t, verifier.Verify(t.Context(), data, signatureJSON))
	require.Error(t, verifier.Verify(t.Context(), []byte("different payload"), signatureJSON))
}

func TestSigstoreConstructorsDoNotMutateDefaultOptions(t *testing.T) {
	signerDefaults := *signeropts.DefaultOptions
	verifierDefaults := *verifieropts.DefaultOptions
	t.Cleanup(func() {
		*signeropts.DefaultOptions = signerDefaults
		*verifieropts.DefaultOptions = verifierDefaults
	})

	NewSigner(signeropts.WithFulcioURL("https://fulcio.example.test"))
	NewVerifierFromIdentityAndIssuer(
		testIdentity,
		testIssuer,
		verifieropts.WithRekorURL("https://rekor.example.test"),
	)

	require.Equal(t, signerDefaults, *signeropts.DefaultOptions)
	require.Equal(t, verifierDefaults, *verifieropts.DefaultOptions)
}

func TestSigstoreKeyIdentifiers(t *testing.T) {
	verifier := NewVerifierFromIdentityAndIssuer(testIdentity, testIssuer)
	keyID, err := verifier.KeyID()
	require.NoError(t, err)
	require.Equal(t, testIdentity+"::"+testIssuer, keyID)

	signer := NewSigner()
	signer.identity = testIdentity
	signer.issuer = testIssuer
	keyID, err = signer.KeyID()
	require.NoError(t, err)
	require.Equal(t, testIdentity+"::"+testIssuer, keyID)

	key, err := signer.MetadataKey()
	require.NoError(t, err)
	require.Equal(t, keyID, key.KeyID)
	require.Equal(t, KeyType, key.KeyType)
	require.Equal(t, KeyScheme, key.Scheme)
	require.Equal(t, testIdentity, key.KeyVal.Identity)
	require.Equal(t, testIssuer, key.KeyVal.Issuer)
}
