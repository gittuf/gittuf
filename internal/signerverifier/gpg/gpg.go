// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gpg

import (
	"bytes"
	"context"
	"crypto"
	"fmt"
	"strings"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/secure-systems-lab/go-securesystemslib/signerverifier"
)

const KeyType = "gpg"

// Verifier is a dsse.Verifier implementation for GPG keys.
type Verifier struct {
	keyID   string
	gpgKey  openpgp.Key
	keyring openpgp.EntityList
}

// NewVerifierFromKey creates a new Verifier from SSlibKey of type GPG.
func NewVerifierFromKey(key *signerverifier.SSLibKey) (*Verifier, error) {
	keyring, err := openpgp.ReadArmoredKeyRing(bytes.NewReader([]byte(key.KeyVal.Public)))
	if err != nil {
		return nil, fmt.Errorf("failed to parse GPG key: %w", err)
	}
	// entity := keyring[0]
	return &Verifier{
		keyID:   key.KeyID,
		gpgKey:  entity.PrimaryKey, // TODO: not sure how to get the openpgp.Key instance here, or if it should be a required field for Verifier?
		keyring: keyring,
	}, nil
}

// Verify implements the dsse.Verifier.Verify interface for GPG keys.
func (v *Verifier) Verify(_ context.Context, data []byte, sig []byte) error {
	sigReader := bytes.NewReader(sig)
	// TODO: can we assume sig is in armored format?
	_, err := openpgp.CheckArmoredDetachedSignature(v.keyring, bytes.NewReader(data), sigReader, nil)
	if err != nil {
		return fmt.Errorf("failed to verify gpg signature: %w", err)
	}
	return nil
}

// KeyID implements the dsse.Verifier.KeyID interface for GPG keys.
// FIXME: consider removing error in interface; a dsse.Verifier needs a keyid
func (v *Verifier) KeyID() (string, error) {
	return v.keyID, nil
}

// Public implements the dsse.Verifier.Public interface for GPG keys.
// FIXME: consider removing in interface, "Verify()" is all that's needed
func (v *Verifier) Public() crypto.PublicKey {
	return v.gpgKey.PublicKey
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
