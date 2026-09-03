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
	"sync"
	"time"

	"github.com/gittuf/gittuf/internal/signerverifier/common"
	"github.com/gittuf/gittuf/internal/signerverifier/gpg"
	"github.com/gittuf/gittuf/internal/signerverifier/sigstore"
	sslibsvssh "github.com/gittuf/gittuf/internal/signerverifier/ssh"
	"github.com/secure-systems-lab/go-securesystemslib/signerverifier"
	"github.com/sigstore/cosign/v3/pkg/cosign"
	gitsignVerifier "github.com/sigstore/gitsign/pkg/git"
	gitsignRekor "github.com/sigstore/gitsign/pkg/rekor"
	sigstoreroot "github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/tuf"
)

const rekorPublicGoodInstance = "https://rekor.sigstore.dev"

// trustedRootFetchTimeout bounds the TUF fetch of the Sigstore trusted root.
const trustedRootFetchTimeout = 30 * time.Second

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

// The Sigstore trusted root is fetched over the network via TUF. A single
// gittuf operation can verify many signatures -- an RSL walk checks every
// signed commit in range, trying each principal's keys in turn -- so the
// fetch and the certificate pools derived from it are cached for the lifetime
// of the process. Only successful fetches are cached, leaving a transient
// network failure to be retried by the next verification.
var (
	trustedRootMutex       sync.Mutex
	cachedTrustedRoot      *sigstoreroot.TrustedRoot
	cachedRootPool         *x509.CertPool
	cachedIntermediatePool *x509.CertPool
)

// getCachedTrustedRootPools returns the root and intermediate certificate
// pools built from the Sigstore trusted root, fetching the trusted root on
// first use.
func getCachedTrustedRootPools() (*x509.CertPool, *x509.CertPool, error) {
	trustedRootMutex.Lock()
	if cachedTrustedRoot != nil {
		root, intermediate := cachedRootPool, cachedIntermediatePool
		trustedRootMutex.Unlock()
		return root, intermediate, nil
	}
	trustedRootMutex.Unlock()

	// The fetch is a network round-trip, so it is performed without the lock
	// held. Concurrent callers may each fetch; the first to finish populates
	// the cache and the others discard their result below.
	trustedRoot, root, intermediate, err := fetchTrustedRootPools()
	if err != nil {
		return nil, nil, err
	}

	trustedRootMutex.Lock()
	defer trustedRootMutex.Unlock()

	// Another caller may have populated the cache while the fetch was in
	// flight. Prefer what is already cached so every caller sees the same
	// pools.
	if cachedTrustedRoot != nil {
		return cachedRootPool, cachedIntermediatePool, nil
	}

	cachedTrustedRoot = trustedRoot
	cachedRootPool = root
	cachedIntermediatePool = intermediate

	return root, intermediate, nil
}

// fetchTrustedRootPools fetches the Sigstore trusted root over TUF and builds
// the root and intermediate certificate pools from it. It performs no caching
// of its own.
func fetchTrustedRootPools() (*sigstoreroot.TrustedRoot, *x509.CertPool, *x509.CertPool, error) {
	// The result is memoized for the process, so the fetch must not be bound
	// to the context of whichever verification happens to trigger it first.
	// The timeout keeps an unresponsive TUF repository from stalling
	// verification indefinitely.
	ctx, cancel := context.WithTimeout(context.Background(), trustedRootFetchTimeout)
	defer cancel()

	trustedRoot, err := sigstoreroot.FetchTrustedRootWithOptions(tuf.DefaultOptions().WithContext(ctx))
	if err != nil {
		return nil, nil, nil, err
	}

	// The trusted root carries each Fulcio CA's root and intermediates
	// together, so both pools are built from the same walk.
	root := x509.NewCertPool()
	intermediate := x509.NewCertPool()
	for _, ca := range trustedRoot.FulcioCertificateAuthorities() {
		fulcioCA, ok := ca.(*sigstoreroot.FulcioCertificateAuthority)
		if !ok {
			return nil, nil, nil, fmt.Errorf("unexpected Fulcio CA type %T", ca)
		}

		if fulcioCA.Root != nil {
			root.AddCert(fulcioCA.Root)
		}
		for _, cert := range fulcioCA.Intermediates {
			intermediate.AddCert(cert)
		}
	}

	return trustedRoot, root, intermediate, nil
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
		root, intermediate, err := getCachedTrustedRootPools()
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
			"-----BEGIN SSH SIGNATURE-----",
			// gitsign (Sigstore) armors its CMS signature under this header.
			"-----BEGIN SIGNED MESSAGE-----":
			count++
		}
	}
	return count
}
