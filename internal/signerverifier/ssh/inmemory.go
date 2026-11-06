// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package ssh

import (
	"bytes"
	"context"
	"crypto"
	"fmt"

	"github.com/hiddeco/sshsig"
	"golang.org/x/crypto/ssh"
)

// InMemorySigner is a dsse.Signer implementation backed by an in-memory SSH
// signer. Unlike Signer, it never shells out to ssh-keygen and needs no key
// file on disk. It produces the same armored sshsig signatures under
// SigNamespace, so its output verifies against Verifier.
type InMemorySigner struct {
	signer ssh.Signer
	*Verifier
}

// NewSignerFromCryptoSigner creates an InMemorySigner from a crypto.Signer
// holding a key type supported by SSH, such as ed25519, ECDSA, or RSA.
func NewSignerFromCryptoSigner(key crypto.Signer) (*InMemorySigner, error) {
	if key == nil {
		return nil, fmt.Errorf("key must not be nil")
	}

	sshSigner, err := ssh.NewSignerFromSigner(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create ssh signer: %w", err)
	}

	verifier, err := NewVerifierFromKey(newSSHKey(sshSigner.PublicKey(), ""))
	if err != nil {
		return nil, err
	}

	return &InMemorySigner{signer: sshSigner, Verifier: verifier}, nil
}

// Sign implements the dsse.Signer.Sign interface for in-memory SSH keys.
func (s *InMemorySigner) Sign(_ context.Context, data []byte) ([]byte, error) {
	signature, err := sshsig.Sign(bytes.NewReader(data), s.signer, sshsig.HashSHA512, SigNamespace)
	if err != nil {
		return nil, fmt.Errorf("failed to sign using ssh key: %w", err)
	}

	return sshsig.Armor(signature), nil
}
