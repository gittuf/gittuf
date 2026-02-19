// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/gittuf/gittuf/internal/signerverifier/common"
	"github.com/gittuf/gittuf/internal/signerverifier/sigstore"
	sslibsvssh "github.com/gittuf/gittuf/internal/signerverifier/ssh"
	"github.com/hiddeco/sshsig"
	"github.com/secure-systems-lab/go-securesystemslib/signerverifier"
	"github.com/sigstore/cosign/v3/pkg/cosign"
	gitsignVerifier "github.com/sigstore/gitsign/pkg/git"
	gitsignRekor "github.com/sigstore/gitsign/pkg/rekor"
	"github.com/sigstore/sigstore/pkg/fulcioroots"
	"golang.org/x/crypto/ssh"
)

const (
	rekorPublicGoodInstance           = "https://rekor.sigstore.dev"
	namespaceSSHSignature      string = "git"
	gpgPrivateKeyPEMHeader     string = "PGP PRIVATE KEY"
	opensshPrivateKeyPEMHeader string = "OPENSSH PRIVATE KEY"
	rsaPrivateKeyPEMHeader     string = "RSA PRIVATE KEY"
	genericPrivateKeyPEMHeader string = "PRIVATE KEY"
	signingFormatGPG           string = "gpg"
	signingFormatSSH           string = "ssh"
)

var (
	ErrNotCommitOrTag             = errors.New("invalid object type, expected commit or tag for signature verification")
	ErrSigningKeyNotSpecified     = errors.New("signing key not specified in git config")
	ErrUnknownSigningMethod       = errors.New("unknown signing method (not one of gpg, ssh, x509)")
	ErrIncorrectVerificationKey   = errors.New("incorrect key provided to verify signature")
	ErrVerifyingSigstoreSignature = errors.New("unable to verify Sigstore signature")
	ErrVerifyingSSHSignature      = errors.New("unable to verify SSH signature")
	ErrInvalidSignature           = errors.New("unable to parse signature / signature has unexpected header")
)

// CanSign inspects the Git configuration to determine if commit / tag signing
// is possible.
func (r *Repository) CanSign() error {
	config, err := r.GetGitConfig()
	if err != nil {
		return err
	}

	// Format is one of GPG, SSH, X509
	format := getSigningMethod(config)

	// If format is GPG or X509, the signing key parameter is optional
	// However, for SSH, the signing key must be set
	if format == signingFormatSSH {
		keyInfo := getSigningKeyInfo(config)
		if keyInfo == "" {
			return ErrSigningKeyNotSpecified
		}
	}

	return nil
}

// VerifySignature verifies the cryptographic signature associated with the
// specified object. The `objectID` must point to a Git commit or tag object.
func (r *Repository) VerifySignature(ctx context.Context, objectID Hash, key *signerverifier.SSLibKey) error {
	if err := r.ensureIsCommit(objectID); err == nil {
		return r.verifyCommitSignature(ctx, objectID, key)
	}

	if err := r.ensureIsTag(objectID); err == nil {
		return r.verifyTagSignature(ctx, objectID, key)
	}

	return ErrNotCommitOrTag
}

func signGitObjectUsingKey(contents, pemKeyBytes []byte) (string, error) {
	block, _ := pem.Decode(pemKeyBytes)
	if block == nil {
		// openpgp implements its own armor-decode method, pem.Decode considers
		// the input invalid. We haven't tested if this is universal, so in case
		// pem.Decode does succeed on a GPG key, we catch it below.
		return signGitObjectUsingGPGKey(contents, pemKeyBytes)
	}

	switch block.Type {
	case gpgPrivateKeyPEMHeader:
		return signGitObjectUsingGPGKey(contents, pemKeyBytes)
	case opensshPrivateKeyPEMHeader, rsaPrivateKeyPEMHeader, genericPrivateKeyPEMHeader:
		return signGitObjectUsingSSHKey(contents, pemKeyBytes)
	}

	return "", ErrUnknownSigningMethod
}

func signGitObjectUsingGPGKey(contents, pemKeyBytes []byte) (string, error) {
	reader := bytes.NewReader(contents)

	keyring, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(pemKeyBytes))
	if err != nil {
		return "", err
	}

	sig := new(strings.Builder)
	if err := openpgp.ArmoredDetachSign(sig, keyring[0], reader, nil); err != nil {
		return "", err
	}

	return sig.String(), nil
}

func signGitObjectUsingSSHKey(contents, pemKeyBytes []byte) (string, error) {
	signer, err := ssh.ParsePrivateKey(pemKeyBytes)
	if err != nil {
		return "", err
	}

	sshSig, err := sshsig.Sign(bytes.NewReader(contents), signer, sshsig.HashSHA512, namespaceSSHSignature)
	if err != nil {
		return "", err
	}

	sigBytes := sshsig.Armor(sshSig)

	return string(sigBytes), nil
}

// verifyGitsignSignature handles the Sigstore-specific workflow involved in
// verifying commit or tag signatures issued by gitsign.
func verifyGitsignSignature(ctx context.Context, repo *Repository, key *signerverifier.SSLibKey, data, signature []byte) error {
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

	rekorURL := rekorPublicGoodInstance
	// Check git config to see if rekor server must be overridden
	config, err := repo.GetGitConfig()
	if err != nil {
		return errors.Join(ErrVerifyingSigstoreSignature, err)
	}
	if configValue, has := config[sigstore.GitConfigRekor]; has {
		slog.Debug(fmt.Sprintf("Using '%s' as Rekor instance...", configValue))
		rekorURL = configValue
	}

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

func getSigningMethod(gitConfig map[string]string) string {
	format, ok := gitConfig["gpg.format"]
	if !ok {
		return signingFormatGPG // default to gpg
	}
	return format
}

func getSigningKeyInfo(gitConfig map[string]string) string {
	keyInfo, ok := gitConfig["user.signingkey"]
	if !ok {
		return ""
	}
	return keyInfo
}
