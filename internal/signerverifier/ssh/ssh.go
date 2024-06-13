// SPDX-License-Identifier: Apache-2.0
package ssh

import (
	"bytes"
	"context"
	"crypto"
	"encoding/base64"
	"fmt"
	"os/exec"
	"strings"

	sv "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/signerverifier"
	"github.com/hiddeco/sshsig"
	"golang.org/x/crypto/ssh"
)

const (
	SSHSigNamespace = "git"
	SSHKeyType      = "ssh"
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
	if err := sshsig.Verify(message, signature, v.sshKey, hash, SSHSigNamespace); err != nil {
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
	cmd := exec.Command("ssh-keygen", "-Y", "sign", "-n", SSHSigNamespace, "-f", s.Path) //nolint:gosec

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
func NewKeyFromFile(path string) (*sv.SSLibKey, error) {
	cmd := exec.Command("ssh-keygen", "-m", "rfc4716", "-e", "-f", path)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run command %v: %w", cmd, err)
	}
	sshPub, err := parseSSH2Key(string(output))
	if err != nil {
		return nil, fmt.Errorf("failed to parse SSH2 key: %w", err)
	}

	return &sv.SSLibKey{
		KeyID:   ssh.FingerprintSHA256(sshPub),
		KeyType: SSHKeyType,
		Scheme:  sshPub.Type(),
		KeyVal: sv.KeyVal{
			Public: base64.StdEncoding.EncodeToString(sshPub.Marshal()),
		},
	}, nil
}

// NewVerifierFromKey creates a new Verifier from SSlibKey of type ssh.
func NewVerifierFromKey(key *sv.SSLibKey) (*Verifier, error) {
	if key.KeyType != SSHKeyType {
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
