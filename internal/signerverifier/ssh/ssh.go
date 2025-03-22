// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package ssh

import (
	"bytes"
	"context"
	"crypto"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hiddeco/sshsig"
	"github.com/secure-systems-lab/go-securesystemslib/signerverifier"
	"golang.org/x/crypto/ssh"
)

const (
	SigNamespace = "git"
	KeyType      = "ssh"
)

// Verifier is a dsse.Verifier implementation for SSH keys.
type Verifier struct {
	keyID  string
	sshKey ssh.PublicKey
}

// Verify implements the dsse.Verifier.Verify interface for SSH keys.
func (v *Verifier) Verify(_ context.Context, data []byte, sig []byte) error {
	signature, err := sshsig.Unarmor(sig)
	if err != nil {
		return fmt.Errorf("failed to parse ssh signature: %w", err)
	}

	message := bytes.NewReader(data)

	// ssh-keygen uses sha512 to sign with **any*** key
	hash := sshsig.HashSHA512
	if err := sshsig.Verify(message, signature, v.sshKey, hash, SigNamespace); err != nil {
		return fmt.Errorf("failed to verify ssh signature: %w", err)
	}

	return nil
}

// KeyID implements the dsse.Verifier.KeyID interface for SSH keys.
// FIXME: consider removing error in interface; a dsse.Verifier needs a keyid
func (v *Verifier) KeyID() (string, error) {
	return v.keyID, nil
}

// Public implements the dsse.Verifier.Public interface for SSH keys.
// FIXME: consider removing in interface, "Verify()" is all that's needed
func (v *Verifier) Public() crypto.PublicKey {
	return v.sshKey.(ssh.CryptoPublicKey).CryptoPublicKey()
}

func (v *Verifier) MetadataKey() *signerverifier.SSLibKey {
	return newSSHKey(v.sshKey, v.keyID)
}

// Signer is a dsse.Signer implementation for SSH keys.
type Signer struct {
	Path string
	*Verifier
}

// Sign implements the dsse.Signer.Sign interface for SSH keys.
// It signs using "s.Path" to a public or private, encrypted or plaintext, rsa,
// ecdsa or ed25519 key file in a format supported by "ssh-keygen". This aligns
// with the git "user.signingKey" option.
// https://git-scm.com/docs/git-config#Documentation/git-config.txt-usersigningKey
func (s *Signer) Sign(_ context.Context, data []byte) ([]byte, error) {
	cmd := exec.Command("ssh-keygen", "-Y", "sign", "-n", SigNamespace, "-f", s.Path) //nolint:gosec

	cmd.Stdin = bytes.NewBuffer(data)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run command %v: %w", cmd, err)
	}

	return output, nil
}

// NewKeyFromFile imports an ssh SSlibKey from the passed path.
// The path can point to a public or private, encrypted or plaintext, rsa,
// ecdsa or ed25519 key file in a format supported by "ssh-keygen". This aligns
// with the git "user.signingKey" option.
// https://git-scm.com/docs/git-config#Documentation/git-config.txt-usersigningKey
func NewKeyFromFile(path string) (*signerverifier.SSLibKey, error) {
	cmd := exec.Command("ssh-keygen", "-m", "rfc4716", "-e", "-f", path)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to run command %v: %w %s", cmd, err, string(output))
	}
	sshPub, err := parseSSH2Key(string(output))
	if err != nil {
		return nil, fmt.Errorf("failed to parse SSH2 key: %w", err)
	}

	return newSSHKey(sshPub, ""), nil
}

// NewKeyFromBytes returns an ssh SSLibKey from the passed bytes. It's meant to
// be used for tests as that's when we directly deal with key bytes.
func NewKeyFromBytes(t *testing.T, keyB []byte) *signerverifier.SSLibKey {
	t.Helper()

	testName := strings.ReplaceAll(t.Name(), " ", "__")
	testName = strings.ReplaceAll(testName, "/", "__")
	testName = strings.ReplaceAll(testName, "\\", "__")
	hash := sha256.Sum256(keyB)
	keyName := fmt.Sprintf("%s-%s", testName, hex.EncodeToString(hash[:]))
	keyPath := filepath.Join(t.TempDir(), keyName)

	if err := os.WriteFile(keyPath, keyB, 0o600); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(keyPath) //nolint:errcheck

	key, err := NewKeyFromFile(keyPath)
	if err != nil {
		t.Fatal(err)
	}

	return key
}

// NewVerifierFromKey creates a new Verifier from SSlibKey of type ssh.
func NewVerifierFromKey(key *signerverifier.SSLibKey) (*Verifier, error) {
	if key.KeyType != KeyType {
		return nil, fmt.Errorf("wrong keyType: %s", key.KeyType)
	}
	sshKey, err := parseSSH2Body(key.KeyVal.Public)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ssh public key material: %w", err)
	}
	return &Verifier{
		keyID:  key.KeyID,
		sshKey: sshKey,
	}, nil
}

// NewSignerFromFile creates an SSH signer from the passed path.
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
		Path:     path,
	}, nil
}

// parseSSH2Body parses a base64-encoded SSH2 wire format key.
func parseSSH2Body(body string) (ssh.PublicKey, error) {
	bodyBytes, err := base64.StdEncoding.DecodeString(body)
	if err != nil {
		return nil, err
	}
	return ssh.ParsePublicKey(bodyBytes)
}

// parseSSH2Key parses a SSH2 public key as defined in RFC4716 (section 3.)
// NOTE:
// - only supports "\n" as line termination character
// - does not validate line length, or header tag or value format
// - discards headers
func parseSSH2Key(data string) (ssh.PublicKey, error) {
	beginMark := "---- BEGIN SSH2 PUBLIC KEY ----"
	endMark := "---- END SSH2 PUBLIC KEY ----"
	lineSep := "\n"
	headerSep := ":"
	continues := "\\"

	// Normalize and trim newlines
	data = strings.ReplaceAll(data, "\r\n", lineSep)
	data = strings.TrimSpace(data)

	// Strip begin and end markers
	lines := strings.Split(data, lineSep)
	if lines[0] != beginMark {
		return nil, fmt.Errorf("expected '%s' in '%s'", beginMark, lines[0])
	}
	last := len(lines) - 1
	if lines[last] != endMark {
		return nil, fmt.Errorf("expected '%s' in '%s'", endMark, lines[last])
	}
	lines = lines[1:last]

	// Strip headers
	var i int
	for i = 0; i < len(lines); i++ {
		if strings.Contains(lines[i], headerSep) {
			continue
		}
		// Skip i==1, first line can not be a continued line
		if i > 0 && strings.HasSuffix(lines[i-1], continues) {
			continue
		}
		break
	}

	// Parse key material
	body := strings.Join(lines[i:], "")
	return parseSSH2Body(body)
}

func newSSHKey(key ssh.PublicKey, keyID string) *signerverifier.SSLibKey {
	if keyID == "" {
		keyID = ssh.FingerprintSHA256(key)
	}
	return &signerverifier.SSLibKey{
		KeyID:   keyID,
		KeyType: KeyType,
		Scheme:  key.Type(),
		KeyVal:  signerverifier.KeyVal{Public: base64.StdEncoding.EncodeToString(key.Marshal())},
	}
}
