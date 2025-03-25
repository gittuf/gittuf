// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package sigstore

import (
	"bytes"
	"context"
	"crypto"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	signeropts "github.com/gittuf/gittuf/internal/signerverifier/sigstore/options/signer"
	verifieropts "github.com/gittuf/gittuf/internal/signerverifier/sigstore/options/verifier"
	"github.com/secure-systems-lab/go-securesystemslib/signerverifier"
	protobundle "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
	protocommon "github.com/sigstore/protobuf-specs/gen/pb-go/common/v1"
	"github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/sign"
	sigstoretuf "github.com/sigstore/sigstore-go/pkg/tuf"
	"github.com/sigstore/sigstore-go/pkg/verify"
	"github.com/sigstore/sigstore/pkg/oauthflow"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	KeyType   = "sigstore-oidc"
	KeyScheme = "fulcio"

	ExtensionMimeType = "application/vnd.dev.sigstore.verificationmaterial;version=0.3"

	GitConfigIssuer      = "gitsign.issuer"
	GitConfigClientID    = "gitsign.clientid"
	GitConfigFulcio      = "gitsign.fulcio"
	GitConfigRekor       = "gitsign.rekor"
	GitConfigRedirectURL = "gitsign.redirecturl"

	EnvSigstoreRootFile           = "SIGSTORE_ROOT_FILE"
	EnvSigstoreCTLogPublicKeyFile = "SIGSTORE_CT_LOG_PUBLIC_KEY_FILE"
	EnvSigstoreRekorPublicKey     = "SIGSTORE_REKOR_PUBLIC_KEY"

	sigstoreBundleMimeType = "application/vnd.dev.sigstore.bundle+json;version=0.3"
)

type Verifier struct {
	rekorURL string
	issuer   string
	identity string
	ext      *structpb.Struct
}

func NewVerifierFromIdentityAndIssuer(identity, issuer string, opts ...verifieropts.Option) *Verifier {
	options := verifieropts.DefaultOptions
	for _, fn := range opts {
		fn(options)
	}

	return &Verifier{
		rekorURL: options.RekorURL,
		issuer:   issuer,
		identity: identity,
	}
}

func (v *Verifier) Verify(_ context.Context, data, sig []byte) error {
	// data is PAE(envelope)
	// sig is raw sigBytes
	// extension is set in the verifier

	slog.Debug("Using Sigstore verifier...")

	trustedRoot, privateInstance, err := v.getTUFRoot()
	if err != nil {
		slog.Debug(fmt.Sprintf("Error getting TUF root: %v", err))
		return err
	}
	slog.Debug("Loaded Sigstore instance's root of trust")

	opts := []verify.VerifierOption{
		verify.WithTransparencyLog(1),
		verify.WithIntegratedTimestamps(1),
	}
	if privateInstance {
		// privateInstance requires online verification if rekor is configured
		// using env var rather than TUF.
		// This is because the trusted_root.json delivered via TUF indicates
		// from when the log can be trusted, which we cannot decide (without a
		// custom env var just for that).
		opts = append(opts, verify.WithOnlineVerification())
	}

	sev, err := verify.NewSignedEntityVerifier(trustedRoot, opts...)
	if err != nil {
		slog.Debug(fmt.Sprintf("Error creating signed entity verifier: %v", err))
		return err
	}

	verificationMaterial := new(protobundle.VerificationMaterial)
	extBytes, err := protojson.Marshal(v.ext)
	if err != nil {
		return err
	}
	if err := protojson.Unmarshal(extBytes, verificationMaterial); err != nil {
		slog.Debug(fmt.Sprintf("Error creating verification material: %v", err))
		return err
	}

	messageSignature := new(protocommon.MessageSignature)
	if err := protojson.Unmarshal(sig, messageSignature); err != nil {
		slog.Debug(fmt.Sprintf("Invalid Sigstore signature: %v", err))
		return err
	}

	// create protobuf bundle
	pbBundle := &protobundle.Bundle{
		MediaType:            sigstoreBundleMimeType,
		VerificationMaterial: verificationMaterial,
		Content: &protobundle.Bundle_MessageSignature{
			MessageSignature: messageSignature,
		},
	}

	apiBundle, err := bundle.NewBundle(pbBundle)
	if err != nil {
		slog.Debug(fmt.Sprintf("Unable to create Sigstore bundle for verification: %v", err))
		return err
	}

	expectedIdentity, err := verify.NewShortCertificateIdentity(v.issuer, "", v.identity, "")
	if err != nil {
		slog.Debug(fmt.Sprintf("Unable to create expected identity constraint: %v", err))
		return err
	}

	result, err := sev.Verify(
		apiBundle,
		verify.NewPolicy(
			verify.WithArtifact(bytes.NewBuffer(data)),
			verify.WithCertificateIdentity(expectedIdentity),
		),
	)
	if err != nil {
		slog.Debug(fmt.Sprintf("Unable to verify Sigstore signature: %v", err))
		return err
	}

	slog.Debug(fmt.Sprintf("Verified Sigstore signature issued by '%s' for '%s'", result.VerifiedIdentity.Issuer.Issuer, result.VerifiedIdentity.SubjectAlternativeName.SubjectAlternativeName))
	return nil
}

func (v *Verifier) KeyID() (string, error) {
	return fmt.Sprintf("%s::%s", v.identity, v.issuer), nil
}

func (v *Verifier) Public() crypto.PublicKey {
	// TODO
	return nil
}

func (v *Verifier) SetExtension(ext *structpb.Struct) {
	v.ext = ext
}

func (v *Verifier) ExpectedExtensionKind() string {
	// TODO: versioning?
	return ExtensionMimeType
}

func (v *Verifier) getTUFRoot() (root.TrustedMaterial, bool, error) {
	// The env vars we look at for private sigstore:
	// SIGSTORE_ROOT_FILE -> the Fulcio root
	// SIGSTORE_CT_LOG_PUBLIC_KEY_FILE -> Fulcio's CT Log pubkey
	// SIGSTORE_REKOR_PUBLIC_KEY -> Rekor's pubkey
	// TODO: Support ctlog and tsa
	fulcioRootFilePath := os.Getenv(EnvSigstoreRootFile)
	ctLogPublicKeyFilePath := os.Getenv(EnvSigstoreCTLogPublicKeyFile)
	rekorPublicKeyFilePath := os.Getenv(EnvSigstoreRekorPublicKey)

	if fulcioRootFilePath != "" || ctLogPublicKeyFilePath != "" || rekorPublicKeyFilePath != "" {
		// if any env var is set, require all?
		if fulcioRootFilePath == "" || ctLogPublicKeyFilePath == "" || rekorPublicKeyFilePath == "" {
			return nil, false, fmt.Errorf("partial env var set") // TODO
		}

		slog.Debug("Using environment variables to establish trust for Sigstore instance...")

		fulcioCertAuthorities := []root.CertificateAuthority{}
		cert, err := parsePEMFile(fulcioRootFilePath)
		if err != nil {
			return nil, false, err
		}
		fulcioCertAuthorities = append(fulcioCertAuthorities, *cert)

		rekorPubKeyBytes, err := os.ReadFile(rekorPublicKeyFilePath)
		if err != nil {
			return nil, false, err
		}
		block, _ := pem.Decode(rekorPubKeyBytes)
		if block == nil {
			return nil, false, fmt.Errorf("failed to decode rekor public key")
		}
		rekorKey, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return nil, false, err
		}

		keyHash := sha256.Sum256(block.Bytes)
		keyID := hex.EncodeToString(keyHash[:])

		rekorTransparencyLog := &root.TransparencyLog{
			BaseURL:           v.rekorURL,
			HashFunc:          crypto.SHA256,
			ID:                keyHash[:],
			PublicKey:         rekorKey,
			SignatureHashFunc: crypto.SHA256,
		}
		rekorTransparencyLogs := map[string]*root.TransparencyLog{
			keyID: rekorTransparencyLog,
		}

		// TODO: CT Log
		// TODO TSA

		trustedRoot, err := root.NewTrustedRoot(root.TrustedRootMediaType01, fulcioCertAuthorities, nil, nil, rekorTransparencyLogs)
		return trustedRoot, true, err
	}

	// Use the TUF flow
	// TODO: support custom sigstore TUF root URL

	tufClient, err := sigstoretuf.New(sigstoretuf.DefaultOptions())
	if err != nil {
		return nil, false, err
	}

	trustedRootJSON, err := tufClient.GetTarget("trusted_root.json")
	if err != nil {
		return nil, false, err
	}

	trustedRoot, err := root.NewTrustedRootFromJSON(trustedRootJSON)
	return trustedRoot, false, err
}

type Signer struct {
	issuerURL   string
	clientID    string
	redirectURL string
	fulcioURL   string
	rekorURL    string
	token       string
	*Verifier
}

func NewSigner(opts ...signeropts.Option) *Signer {
	options := signeropts.DefaultOptions
	for _, fn := range opts {
		fn(options)
	}

	return &Signer{
		issuerURL:   options.IssuerURL,
		clientID:    options.ClientID,
		redirectURL: options.RedirectURL,
		fulcioURL:   options.FulcioURL,
		rekorURL:    options.RekorURL,
		Verifier: &Verifier{
			rekorURL: options.RekorURL,
		},
	}
}

func (s *Signer) Sign(_ context.Context, data []byte) ([]byte, error) {
	content := &sign.PlainData{Data: data}

	keypair, err := sign.NewEphemeralKeypair(nil)
	if err != nil {
		return nil, err
	}

	// TODO: support private sigstore by reading config

	opts := sign.BundleOptions{}

	// We reuse the token if it's already been fetched once for this signer
	// object
	// getIDToken also populates the Verifier's identity and issuer pieces
	token, err := s.getIDToken()
	if err != nil {
		return nil, err
	}
	opts.CertificateProviderOptions = &sign.CertificateProviderOptions{IDToken: token}

	fulcio := s.getFulcioInstance()
	opts.CertificateProvider = fulcio

	// TODO: TSA support?

	rekor := s.getRekorInstance()
	opts.TransparencyLogs = append(opts.TransparencyLogs, rekor)

	bundle, err := sign.Bundle(content, keypair, opts)
	if err != nil {
		return nil, err
	}

	bundleJSON, err := protojson.Marshal(bundle)
	if err != nil {
		log.Fatal(err)
	}

	return bundleJSON, nil
}

func (s *Signer) KeyID() (string, error) {
	// verifier can't return error
	verifierKeyID, _ := s.Verifier.KeyID() //nolint:errcheck
	if verifierKeyID == "::" {
		// verifier.identity and verifier.issuer are empty resulting in this
		// return value

		// getIDToken will populate verifier
		_, err := s.getIDToken()
		if err != nil {
			return "", err
		}
	}

	return s.Verifier.KeyID()
}

// MetadataKey returns the securesystemslib representation of the key, used for
// its representation in gittuf metadata.
func (s *Signer) MetadataKey() (*signerverifier.SSLibKey, error) {
	keyID, err := s.KeyID()
	if err != nil {
		return nil, err
	}

	return &signerverifier.SSLibKey{
		KeyID:   keyID,
		KeyType: KeyType,
		Scheme:  KeyScheme,
		KeyVal: signerverifier.KeyVal{
			Identity: s.identity,
			Issuer:   s.issuer,
		},
	}, nil
}

func (s *Signer) getIDToken() (string, error) {
	if s.token == "" {
		// TODO: support client secret?
		token, err := oauthflow.OIDConnect(s.issuerURL, s.clientID, "", s.redirectURL, oauthflow.DefaultIDTokenGetter)
		if err != nil {
			return "", err
		}

		s.token = token.RawString

		// Set identity and issuer pieces
		identity, issuer, err := parseTokenForIdentityAndIssuer(s.token, s.fulcioURL)
		if err != nil {
			return "", err
		}

		s.identity = identity
		s.issuer = issuer
	}

	return s.token, nil
}

func (s *Signer) getFulcioInstance() *sign.Fulcio {
	fulcioOpts := &sign.FulcioOptions{
		BaseURL: s.fulcioURL,
		Timeout: time.Minute,
		Retries: 1,
	}
	return sign.NewFulcio(fulcioOpts)
}

func (s *Signer) getRekorInstance() *sign.Rekor {
	rekorOpts := &sign.RekorOptions{
		BaseURL: s.rekorURL,
		Timeout: 90 * time.Second,
		Retries: 1,
	}
	return sign.NewRekor(rekorOpts)
}
