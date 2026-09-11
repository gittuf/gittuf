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

	if len(keyring) == 0 {
		return nil, fmt.Errorf("no GPG keys found in armored input")
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

// msysPath converts a native Windows path (e.g. `C:\Users\foo`) into the
// POSIX-style form MSYS tools such as Git for Windows' bundled gpg expect
// for absolute paths (e.g. `/c/Users/foo`). See the MSYS2 documentation on
// path conversion: https://www.msys2.org/docs/filesystem-paths/
//
// This intentionally doesn't use filepath.ToSlash: that only rewrites the
// separator native to the OS the code is compiled for, so on a non-Windows
// host (as when this package's tests run on Linux/macOS CI runners) it
// leaves Windows-style backslashes untouched instead of converting them.
func msysPath(path string) string {
	path = strings.ReplaceAll(path, `\`, "/")
	if len(path) >= 2 && path[1] == ':' {
		path = "/" + strings.ToLower(path[:1]) + path[2:]
	}
	return path
}

// gpgIsMSYSBuild reports whether the `gpg` binary that will actually be
// invoked is an MSYS build, such as the one bundled with Git for Windows.
// This matters because MSYS gpg only recognizes POSIX-style absolute paths
// (e.g. `/c/Users/...`): given a native Windows path, it doesn't treat it as
// absolute and silently resolves it relative to gpg's working directory
// instead. A non-MSYS (e.g. Gpg4win) install, on the other hand, expects the
// native Windows path as-is, so callers must not assume one or the other
// merely from being on Windows.
func gpgIsMSYSBuild() bool {
	path, err := exec.LookPath("gpg")
	if err != nil {
		return false
	}

	return isMSYSGPGPath(path)
}

// isMSYSGPGPath reports whether path (as resolved by exec.LookPath("gpg"))
// points at an MSYS build of gpg, such as the one bundled with Git for
// Windows under `Git\usr\bin` or `Git\mingw64\bin`. See
// https://www.msys2.org/docs/environments/ for background on the usr/bin
// vs. mingw64/bin MSYS2 environments Git for Windows is built from.
//
// As in msysPath, this doesn't use filepath.ToSlash, since that would only
// normalize separators when compiled for Windows and this needs to work
// when the package's tests run on any OS.
func isMSYSGPGPath(path string) bool {
	path = strings.ToLower(strings.ReplaceAll(path, `\`, "/"))
	return strings.Contains(path, "/git/usr/bin/gpg") || strings.Contains(path, "/git/mingw64/bin/gpg")
}

// SetupTestGPGHomeDir is a test helper used only to prepare a temporary GPG
// home dir with the specified keys added in.
func SetupTestGPGHomeDir(t *testing.T, privateKeyBytes ...[]byte) {
	// We use os.MkdirTemp because t.TempDir can result in a path
	// that's too long for socket files, used for gpg-agent.
	tmpGpgHomeDir, err := os.MkdirTemp("", "gittuf-gpg-")
	require.Nil(t, err)
	t.Cleanup(func() {
		os.RemoveAll(tmpGpgHomeDir) //nolint:errcheck
	})

	gnupgHome := tmpGpgHomeDir
	if runtime.GOOS == "windows" && gpgIsMSYSBuild() {
		gnupgHome = msysPath(tmpGpgHomeDir)
	}
	t.Setenv("GNUPGHOME", gnupgHome)

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
