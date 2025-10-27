// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gpg

import (
	"bytes"
	"context"
	"crypto"
	"fmt"
	"os/exec"
	"strings"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/secure-systems-lab/go-securesystemslib/signerverifier"
)

const KeyType = "gpg"

// Verifier is a dsse.Verifier implementation for GPG keys.
type Verifier struct {
	keyID  string
	entity *openpgp.Entity
}

// NewVerifierFromKey creates a new verifier from SSlibKey of type GPG.
func NewVerifierFromKey(key *signerverifier.SSLibKey) (*Verifier, error) {
	keyring, err := openpgp.ReadArmoredKeyRing(bytes.NewReader([]byte(key.KeyVal.Public)))
	if err != nil {
		return nil, fmt.Errorf("failed to parse gpg key: %w", err)
	}

	entity := keyring[0]
	return &Verifier{
		keyID:  key.KeyID,
		entity: entity,
	}, nil
}

// Verify implements the dsse.Verifier.Verify interface for GPG keys.
func (v *Verifier) Verify(_ context.Context, data []byte, sig []byte) error {
	sigReader := bytes.NewReader(sig)
	_, err := openpgp.CheckArmoredDetachedSignature(openpgp.EntityList{v.entity}, bytes.NewReader(data), sigReader, nil)
	if err != nil {
		return fmt.Errorf("failed to verify gpg signature: %w", err)
	}
	return nil
}

type Signer struct {
	entity *openpgp.Entity
	*Verifier
}

func (s *Signer) KeyID() (string, error) {
	return s.keyID, nil
}

func NewSignerFromKeyID(keyID string) (*Signer, error) {
	signingKey, err := NewPrivateKeyFromKeyID(keyID)
	if err != nil {
		return nil, err
	}
	pubKeyObj, err := NewPublicKeyFromKeyID(keyID)
	if err != nil {
		return nil, err
	}

	verifier, err := NewVerifierFromKey(pubKeyObj)
	if err != nil {
		return nil, err
	}

	keyring, err := openpgp.ReadArmoredKeyRing(bytes.NewReader([]byte(signingKey.KeyVal.Private)))
	if err != nil {
		return nil, fmt.Errorf("failed to parse private gpg key: %w", err)
	}

	entity := keyring[0]
	return &Signer{
		Verifier: verifier,
		entity:   entity,
	}, nil
}

// NewSignerFromKey creates a new GPG signer from an SSLibKey containing a private key.
// This method requires passing in the verifier from a possibly different keyID.
// When loading a signer using Git config, NewSignerFromKeyID should be used instead.
func NewSignerFromKey(key *signerverifier.SSLibKey, verifier *Verifier) (*Signer, error) {
	keyring, err := openpgp.ReadArmoredKeyRing(bytes.NewReader([]byte(key.KeyVal.Private)))
	if err != nil {
		return nil, fmt.Errorf("failed to parse private gpg key: %w", err)
	}

	entity := keyring[0]
	return &Signer{
		Verifier: verifier,
		entity:   entity,
	}, nil
}

// Sign implements the dsse.Signer.Sign interface for GPG keys.
func (s *Signer) Sign(_ context.Context, data []byte) ([]byte, error) {
	buf := new(bytes.Buffer)
	err := openpgp.ArmoredDetachSign(buf, s.entity, bytes.NewReader(data), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to sign with gpg key: %w", err)
	}

	return buf.Bytes(), nil
}

// KeyID implements the dsse.Verifier.KeyID interface for GPG keys.
// FIXME: consider removing error in interface; a dsse.Verifier needs a keyid
func (v *Verifier) KeyID() (string, error) {
	return v.keyID, nil
}

// Public implements the dsse.Verifier.Public interface for GPG keys.
// FIXME: consider removing in interface, "Verify()" is all that's needed
func (v *Verifier) Public() crypto.PublicKey {
	return v.entity.PrimaryKey.PublicKey
}

// LoadGPGKeyFromBytes returns a signerverifier.SSLibKey for a GPG / PGP key passed in as
// armored bytes. The returned signerverifier.SSLibKey uses the primary key's fingerprint as the
// key ID.
func LoadGPGKeyFromBytes(contents []byte) (*signerverifier.SSLibKey, error) {
	keyring, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(contents))
	if err != nil {
		return nil, err
	}

	// TODO: check if this is correct for subkeys
	fingerprint := fmt.Sprintf("%x", keyring[0].PrimaryKey.Fingerprint)
	key := strings.TrimSpace(string(contents))

	gpgKey := &signerverifier.SSLibKey{
		KeyID:   fingerprint,
		KeyType: KeyType,
		Scheme:  KeyType, // TODO: this should use the underlying key algorithm
		KeyVal: signerverifier.KeyVal{
			Public: key,
		},
	}

	return gpgKey, nil
}

// LoadGPGKeyFromBytes returns a signerverifier.SSLibKey for a GPG / PGP key passed in as
// armored bytes. The returned signerverifier.SSLibKey uses the primary key's fingerprint as the
// key ID.
func LoadGPGPrivKeyFromBytes(contents []byte) (*signerverifier.SSLibKey, error) {
	keyring, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(contents))
	if err != nil {
		return nil, err
	}

	// TODO: check if this is correct for subkeys
	fingerprint := fmt.Sprintf("%x", keyring[0].PrimaryKey.Fingerprint)
	key := strings.TrimSpace(string(contents))

	gpgKey := &signerverifier.SSLibKey{
		KeyID:   fingerprint,
		KeyType: KeyType,
		Scheme:  KeyType, // TODO: this should use the underlying key algorithm
		KeyVal: signerverifier.KeyVal{
			Private: key,
		},
	}

	return gpgKey, nil
}

func NewPublicKeyFromKeyID(keyID string) (*signerverifier.SSLibKey, error) {
	cmd := exec.Command("gpg", "--batch", "--armor", "--export", keyID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to run command %v: %w %s", cmd, err, string(output))
	}

	publicKey := strings.TrimSpace(string(output))
	gpgKey := &signerverifier.SSLibKey{
		KeyID:   keyID,
		KeyType: KeyType,
		Scheme:  KeyType, // TODO: this should use the underlying key algorithm
		KeyVal: signerverifier.KeyVal{
			Public: publicKey,
		},
	}

	return gpgKey, nil
}

func NewPrivateKeyFromKeyID(keyID string) (*signerverifier.SSLibKey, error) {
	cmd := exec.Command("gpg", "--batch", "--armor", "--export-secret-keys", keyID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to run command %v: %w %s", cmd, err, string(output))
	}

	privateKey := strings.TrimSpace(string(output))
	gpgKey := &signerverifier.SSLibKey{
		KeyID:   keyID,
		KeyType: KeyType,
		Scheme:  KeyType, // TODO: this should use the underlying key algorithm
		KeyVal: signerverifier.KeyVal{
			Private: privateKey,
		},
	}

	return gpgKey, nil
}

// getKeyIDFromGitConfig queries the user git config and returns the keyID if it exists.
func getKeyIDFromGitConfig() (string, error) {
	cmd := exec.Command("git", "config", "--get", "user.signingkey")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("unable to read Git config: %w", err)
	}

	keyID := strings.TrimSpace(string(output))
	if keyID == "" {
		return "", fmt.Errorf("no user.signingkey set in git")
	}

	// TODO: check if keyID is a gpg key
	var isGPG bool
	isGPG = true
	if !isGPG {
		return "", fmt.Errorf("user.signingkey is not a GPG key")
	}
	// TODO: keyID may need to be converted to full fingerprint
	return keyID, nil
}
