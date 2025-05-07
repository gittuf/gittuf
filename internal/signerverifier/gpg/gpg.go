// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gpg

import (
	"bytes"
	"context"
	"crypto"
	"fmt"
	"os"
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

// NewVerifierFromKey creates a new erifier from SSlibKey of type GPG.
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
	*Verifier
}

// NewSignerFromFile creates an GPG signer from the passed path.
func NewSignerFromFile(path string) (*Signer, error) {
	keyObj, err := NewKeyFromFile(path)
	if err != nil {
		return nil, err
	}
	verifier, err := NewVerifierFromKey(keyObj)
	if err != nil {
		return nil, err
	}

	return &Signer{
		Verifier: verifier,
	}, nil
}

// Sign implements the dsse.Signer.Sign interface for GPG keys.
func (s *Signer) Sign(_ context.Context, data []byte) ([]byte, error) {
	buf := new(bytes.Buffer)
	err := openpgp.ArmoredDetachSign(buf, s.Verifier.entity, bytes.NewReader(data), nil)
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
	// TODO: might have to handle case where there is more than one entity
	fingerprint := fmt.Sprintf("%x", keyring[0].PrimaryKey.Fingerprint)
	publicKey := strings.TrimSpace(string(contents))

	gpgKey := &signerverifier.SSLibKey{
		KeyID:   fingerprint,
		KeyType: KeyType,
		Scheme:  KeyType, // TODO: this should use the underlying key algorithm
		KeyVal: signerverifier.KeyVal{
			Public: publicKey,
		},
	}

	return gpgKey, nil
}

func NewKeyFromFile(path string) (*signerverifier.SSLibKey, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	keyring, err := openpgp.ReadArmoredKeyRing(file)
	if err != nil {
		return nil, err
	}

	// TODO: might have to handle case where there is more than one entity
	fingerprint := fmt.Sprintf("%x", keyring[0].PrimaryKey.Fingerprint)
	cmd := exec.Command("gpg", "--armor", "--export")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to run command %v: %w %s", cmd, err, string(output))
	}

	publicKey := strings.TrimSpace(string(output))
	gpgKey := &signerverifier.SSLibKey{
		KeyID:   fingerprint,
		KeyType: KeyType,
		Scheme:  KeyType, // TODO: this should use the underlying key algorithm
		KeyVal: signerverifier.KeyVal{
			Public: publicKey,
		},
	}

	return gpgKey, nil
}
