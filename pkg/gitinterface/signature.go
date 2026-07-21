// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"bytes"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/hiddeco/sshsig"
	"golang.org/x/crypto/ssh"
)

const (
	namespaceSSHSignature      string = "git"
	gpgPrivateKeyPEMHeader     string = "PGP PRIVATE KEY"
	opensshPrivateKeyPEMHeader string = "OPENSSH PRIVATE KEY"
	rsaPrivateKeyPEMHeader     string = "RSA PRIVATE KEY"
	genericPrivateKeyPEMHeader string = "PRIVATE KEY"
	signingFormatGPG           string = "gpg"
	signingFormatSSH           string = "ssh"
)

var (
	ErrNotCommitOrTag         = errors.New("invalid object type, expected commit or tag for signature verification")
	ErrSigningKeyNotSpecified = errors.New("signing key not specified in git config")

	// ErrUnknownSigningMethod covers the signing side, in
	// signGitObjectUsingKey. Verification reports
	// gitobject.ErrUnknownSigningMethod instead.
	ErrUnknownSigningMethod = errors.New("unknown signing method (not one of gpg, ssh, x509)")
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

// GetObjectSignature returns the signed payload and the detached signature for
// the specified Git object. The `objectID` must point to a commit or tag
// object. An unsigned object returns an empty signature and no error, and
// verification layers decide how to treat it. For commits the signature is
// read from the header matching the repository's object format (`gpgsig` or
// `gpgsig-sha256`). For tags it is the block Git appends to the payload.
// Signatures containing multiple armored blocks are returned verbatim, and
// callers performing verification must reject them.
func (r *Repository) GetObjectSignature(objectID Hash) ([]byte, []byte, error) {
	if err := r.ensureIsCommit(objectID); err == nil {
		goGitRepo, err := r.GetGoGitRepository()
		if err != nil {
			return nil, nil, fmt.Errorf("error opening repository: %w", err)
		}

		commit, err := goGitRepo.CommitObject(plumbing.NewHash(objectID.String()))
		if err != nil {
			return nil, nil, fmt.Errorf("unable to load commit object: %w", err)
		}

		payload, err := getCommitBytesWithoutSignature(commit)
		if err != nil {
			return nil, nil, fmt.Errorf("unable to encode commit contents: %w", err)
		}

		return payload, []byte(signatureForObjectID(objectID, commit.Signature, commit.SignatureSHA256)), nil
	}

	if err := r.ensureIsTag(objectID); err == nil {
		goGitRepo, err := r.GetGoGitRepository()
		if err != nil {
			return nil, nil, fmt.Errorf("error opening repository: %w", err)
		}

		tag, err := goGitRepo.TagObject(plumbing.NewHash(objectID.String()))
		if err != nil {
			return nil, nil, fmt.Errorf("unable to load tag object: %w", err)
		}

		payload, err := getTagBytesWithoutSignature(tag)
		if err != nil {
			return nil, nil, fmt.Errorf("unable to encode tag contents: %w", err)
		}

		// Git appends tag signatures to the tag payload regardless of the
		// object format, so the signature is always in the Signature field.
		return payload, []byte(tag.Signature), nil
	}

	return nil, nil, ErrNotCommitOrTag
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

// signatureForObjectID selects the signature stored for a Git commit based on
// its hash algorithm, identified by the OID length. SHA-256 commits store
// their signature under the `gpgsig-sha256` header (go-git's SignatureSHA256),
// SHA-1 commits under `gpgsig` (Signature). Tags are not covered here: Git
// appends tag signatures to the tag payload regardless of the object format.
func signatureForObjectID(objectID Hash, signature, signatureSHA256 string) string {
	if objectID.IsSHA256() {
		return signatureSHA256
	}
	return signature
}
