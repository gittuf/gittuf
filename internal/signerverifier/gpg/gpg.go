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
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ProtonMail/go-crypto/openpgp"
	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/secure-systems-lab/go-securesystemslib/signerverifier"
	"github.com/stretchr/testify/require"
)

const (
	KeyType = "gpg"

	defaultGPGProgram = "gpg"
)

// Verifier is a dsse.Verifier implementation for GPG keys.
type Verifier struct {
	metadataKey *signerverifier.SSLibKey
	keyID       string
	entity      *openpgp.Entity
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

func (v *Verifier) MetadataKey() *signerverifier.SSLibKey {
	return v.metadataKey
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

// NewVerifierFromKey creates a new verifier from SSLibKey of type GPG.
func NewVerifierFromKey(key *signerverifier.SSLibKey) (*Verifier, error) {
	keyring, err := openpgp.ReadArmoredKeyRing(bytes.NewReader([]byte(key.KeyVal.Public)))
	if err != nil {
		return nil, fmt.Errorf("failed to parse gpg key: %w", err)
	}

	entity := keyring[0]
	return &Verifier{
		metadataKey: key,
		keyID:       key.KeyID,
		entity:      entity,
	}, nil
}

type Signer struct {
	*Verifier
	program string
}

func (s *Signer) KeyID() (string, error) {
	return s.keyID, nil
}

// Sign implements the dsse.Signer.Sign interface for GPG keys.
func (s *Signer) Sign(_ context.Context, data []byte) ([]byte, error) {
	cmd := exec.Command(s.program, "--status-fd=2", "-bsau", s.keyID) //nolint:gosec

	cmd.Stdin = bytes.NewBuffer(data)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("unable to run command %v: %w", cmd, err)
	}
	return output, nil
}

func NewSignerFromKeyID(keyID string, opts ...SignerOption) (*Signer, error) {
	options := &SignerOptions{program: defaultGPGProgram}
	for _, fn := range opts {
		fn(options)
	}

	pubKeyObj, err := getPublicKeyForKeyID(keyID, options.program)
	if err != nil {
		return nil, err
	}

	verifier, err := NewVerifierFromKey(pubKeyObj)
	if err != nil {
		return nil, err
	}

	return &Signer{Verifier: verifier, program: options.program}, nil
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

func getPublicKeyForKeyID(keyID, program string) (*signerverifier.SSLibKey, error) {
	cmd := exec.Command(program, "--batch", "--armor", "--export", keyID) //nolint:gosec
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to run command %v: %w %s", cmd, err, string(output))
	}

	return LoadGPGKeyFromBytes(output)
}

// SetupTestGPGHomeDir is a test helper used only to prepare a temporary GPG
// home dir with the specified keys added in.
func SetupTestGPGHomeDir(t *testing.T, privateKeyBytes ...[]byte) {
	if runtime.GOOS == "windows" {
		t.Skip("TODO: test gpg keys for metadata on Windows")
	}

	// We use os.MkdirTemp because t.TempDir can result in a path
	// that's too long for socket files, used for gpg-agent.
	tmpGpgHomeDir, err := os.MkdirTemp("", "gittuf-gpg-")
	require.Nil(t, err)
	t.Cleanup(func() {
		os.RemoveAll(tmpGpgHomeDir) //nolint:errcheck
	})

	t.Setenv("GNUPGHOME", tmpGpgHomeDir)

	gpgAgentConfPath := filepath.Join(tmpGpgHomeDir, "gpg-agent.conf")
	if err := os.WriteFile(gpgAgentConfPath, artifacts.GPGAgentConf, 0o600); err != nil {
		t.Fatal(err)
	}

	for _, keyBytes := range privateKeyBytes {
		cmd := exec.Command("gpg", "--import")
		cmd.Stdin = bytes.NewReader(keyBytes)

		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatal(fmt.Errorf("%w: %s", err, string(output)))
		}
	}
}

type SignerOptions struct {
	program string
}

type SignerOption func(*SignerOptions)

func WithGPGProgram(program string) SignerOption {
	return func(opts *SignerOptions) {
		opts.program = program
	}
}
