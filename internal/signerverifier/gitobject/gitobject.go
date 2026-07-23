// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

// Package gitobject verifies signatures over Git object payloads. It operates
// purely on bytes and carries no dependency on any repository implementation.
// Callers extract the signed payload and detached signature from their storage
// layer, for example gitinterface's Repository.GetObjectSignature.
package gitobject

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/gittuf/gittuf/internal/signerverifier/common"
	"github.com/gittuf/gittuf/internal/signerverifier/gpg"
	"github.com/gittuf/gittuf/internal/signerverifier/sigstore"
	sslibsvssh "github.com/gittuf/gittuf/internal/signerverifier/ssh"
	"github.com/secure-systems-lab/go-securesystemslib/signerverifier"
	"github.com/sigstore/cosign/v3/pkg/cosign"
	gitsignVerifier "github.com/sigstore/gitsign/pkg/git"
	gitsignRekor "github.com/sigstore/gitsign/pkg/rekor"
	"github.com/sigstore/sigstore/pkg/fulcioroots"
)

const rekorPublicGoodInstance = "https://rekor.sigstore.dev"

var (
	ErrUnknownSigningMethod       = errors.New("unknown signing method (not one of gpg, ssh, x509)")
	ErrIncorrectVerificationKey   = errors.New("incorrect key provided to verify signature")
	ErrVerifyingSigstoreSignature = errors.New("unable to verify Sigstore signature")
	ErrVerifyingSSHSignature      = errors.New("unable to verify SSH signature")
	ErrMultipleSignatures         = errors.New("object has multiple signatures")
)

type options struct {
	rekorURL string
}

// Option configures Verify.
type Option func(*options)

// WithRekorURL overrides the Rekor instance used for Sigstore verification.
// Callers typically resolve this from the repository's gpg.x509.rekor git
// config. The default is the Rekor public good instance.
func WithRekorURL(url string) Option {
	return func(o *options) {
		o.rekorURL = url
	}
}

// Verify checks the signature over a Git object payload with the provided
// key. The payload and signature are typically obtained from gitinterface's
// Repository.GetObjectSignature. The error contract callers rely on:
// ErrUnknownSigningMethod is returned for key types this package cannot
// handle. errors.Is(err, ErrIncorrectVerificationKey) holds for any
// verification failure, including infrastructure failures in the SSH and
// Sigstore paths, which additionally match ErrVerifyingSSHSignature or
// ErrVerifyingSigstoreSignature respectively so callers can distinguish
// them. A signature with multiple armored blocks additionally matches
// ErrMultipleSignatures.
func Verify(ctx context.Context, key *signerverifier.SSLibKey, payload, signature []byte, opts ...Option) error {
	o := &options{rekorURL: rekorPublicGoodInstance}
	for _, fn := range opts {
		fn(o)
	}

	if signatureBlockCount(string(signature)) > 1 {
		return errors.Join(ErrIncorrectVerificationKey, ErrMultipleSignatures)
	}

	switch key.KeyType {
	case gpg.KeyType:
		verifier, err := gpg.NewVerifierFromKey(key)
		if err != nil {
			return errors.Join(ErrIncorrectVerificationKey, err)
		}
		// TODO: normalize error joining across branches. The gpg branch
		// discards the underlying error, unlike the ssh and sigstore ones.
		if err := verifier.Verify(ctx, payload, signature); err != nil {
			return ErrIncorrectVerificationKey
		}

		return nil
	case sslibsvssh.KeyType:
		if err := verifySSHKeySignature(ctx, key, payload, signature); err != nil {
			return errors.Join(ErrIncorrectVerificationKey, err)
		}

		return nil
	case sigstore.KeyType:
		if err := verifyGitsignSignature(ctx, key, payload, signature, o.rekorURL); err != nil {
			return errors.Join(ErrIncorrectVerificationKey, err)
		}

		return nil
	default:
		return ErrUnknownSigningMethod
	}
}

// verifyGitsignSignature handles the Sigstore-specific workflow involved in
// verifying commit or tag signatures issued by gitsign.
func verifyGitsignSignature(ctx context.Context, key *signerverifier.SSLibKey, data, signature []byte, rekorURL string) error {
	checkOpts := &cosign.CheckOpts{
		Identities: []cosign.Identity{{
			Issuer:  key.KeyVal.Issuer,
			Subject: key.KeyVal.Identity,
		}},
	}

	var verifier *gitsignVerifier.CertVerifier
	sigstoreRootFilePath := os.Getenv(sigstore.EnvSigstoreRootFile)
	if sigstoreRootFilePath == "" {
		root, err := fulcioroots.Get()
		if err != nil {
			return errors.Join(ErrVerifyingSigstoreSignature, err)
		}
		intermediate, err := fulcioroots.GetIntermediates()
		if err != nil {
			return errors.Join(ErrVerifyingSigstoreSignature, err)
		}

		checkOpts.RootCerts = root
		checkOpts.IntermediateCerts = intermediate

		verifier, err = gitsignVerifier.NewCertVerifier(
			gitsignVerifier.WithRootPool(root),
			gitsignVerifier.WithIntermediatePool(intermediate),
		)
		if err != nil {
			return errors.Join(ErrVerifyingSigstoreSignature, err)
		}
	} else {
		slog.Debug("Using environment variables to establish trust for Sigstore instance...")
		rootCerts, err := common.LoadCertsFromPath(sigstoreRootFilePath)
		if err != nil {
			return errors.Join(ErrVerifyingSigstoreSignature, err)
		}
		root := x509.NewCertPool()
		for _, cert := range rootCerts {
			root.AddCert(cert)
		}

		checkOpts.RootCerts = root

		verifier, err = gitsignVerifier.NewCertVerifier(
			gitsignVerifier.WithRootPool(root),
		)
		if err != nil {
			return errors.Join(ErrVerifyingSigstoreSignature, err)
		}
	}

	verifiedCert, err := verifier.Verify(ctx, data, signature, true)
	if err != nil {
		return ErrIncorrectVerificationKey
	}

	slog.Debug(fmt.Sprintf("Using '%s' as Rekor instance...", rekorURL))

	// gitsignRekor.NewWithOptions invokes cosign.GetRekorPubs which looks at
	// the env var, so we don't have to do anything here
	rekor, err := gitsignRekor.NewWithOptions(ctx, rekorURL)
	if err != nil {
		return errors.Join(ErrVerifyingSigstoreSignature, err)
	}

	checkOpts.RekorClient = rekor.Rekor
	checkOpts.RekorPubKeys = rekor.PublicKeys()

	// cosign.GetCTLogPubs already looks at the env var, so we don't have to do
	// anything here
	ctPub, err := cosign.GetCTLogPubs(ctx)
	if err != nil {
		return errors.Join(ErrVerifyingSigstoreSignature, err)
	}

	checkOpts.CTLogPubKeys = ctPub

	if _, err := cosign.ValidateAndUnpackCert(verifiedCert, checkOpts); err != nil {
		return errors.Join(ErrIncorrectVerificationKey, err)
	}

	return nil
}

// verifySSHKeySignature verifies Git signatures issued by SSH keys.
func verifySSHKeySignature(ctx context.Context, key *signerverifier.SSLibKey, data, signature []byte) error {
	verifier, err := sslibsvssh.NewVerifierFromKey(key)
	if err != nil {
		return errors.Join(ErrVerifyingSSHSignature, err)
	}

	if err := verifier.Verify(ctx, data, signature); err != nil {
		return errors.Join(ErrVerifyingSSHSignature, err)
	}

	return nil
}

// signatureBlockCount reports how many armored signature blocks appear in a
// signature. Verification rejects values carrying more than one block, which
// are ambiguous.
func signatureBlockCount(signature string) int {
	count := 0
	for line := range strings.SplitSeq(signature, "\n") {
		switch strings.TrimSpace(line) {
		case "-----BEGIN PGP SIGNATURE-----",
			"-----BEGIN PGP MESSAGE-----",
			"-----BEGIN SSH SIGNATURE-----":
			count++
		}
	}
	return count
}
