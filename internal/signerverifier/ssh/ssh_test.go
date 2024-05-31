// SPDX-License-Identifier: Apache-2.0
package ssh

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/stretchr/testify/assert"
)

// Basic smoke test for ssh package for all supported keys
func TestSSH(t *testing.T) {
	keyidRSA := "SHA256:ESJezAOo+BsiEpddzRXS6+wtF16FID4NCd+3gj96rFo"
	keyidECDSA := "SHA256:oNYBImx035m3rl1Sn/+j5DPrlS9+zXn7k3mjNrC5eto"
	keyidEd25519 := "SHA256:cewFulOIcROWnolPTGEQXG4q7xvLIn3kNTCMqdfoP4E"

	tests := []struct {
		keyName  string
		keyBytes []byte
		keyID    string
	}{
		{"rsa", artifacts.SSHRSAPrivate, keyidRSA},
		{"rsa.pub", artifacts.SSHRSAPublicSSH, keyidRSA},
		{"rsa_enc", artifacts.SSHRSAPrivateEnc, keyidRSA},
		{"rsa_enc.pub", artifacts.SSHRSAPublicSSH, keyidRSA},
		{"ecdsa", artifacts.SSHECDSAPrivate, keyidECDSA},
		{"ecdsa.pub", artifacts.SSHECDSAPublicSSH, keyidECDSA},
		{"ecdsa_enc", artifacts.SSHECDSAPrivateEnc, keyidECDSA},
		{"ecdsa_enc.pub", artifacts.SSHECDSAPublicSSH, keyidECDSA},
		{"ed25519", artifacts.SSHED25519Private, keyidEd25519},
		{"ed25519.pub", artifacts.SSHED25519PublicSSH, keyidEd25519},
		{"ed25519_enc", artifacts.SSHED25519PrivateEnc, keyidEd25519},
		{"ed25519_enc.pub", artifacts.SSHED25519PublicSSH, keyidEd25519},
	}
	// Setup tests
	tmpDir := t.TempDir()
	// Write script to mock password prompt

	scriptPath := filepath.Join(tmpDir, "askpass.sh")
	if err := os.WriteFile(scriptPath, artifacts.AskpassScript, 0o500); err != nil { //nolint:gosec
		t.Fatal(err)
	}

	// Write test key pairs to temp dir with permissions required by ssh-keygen
	for _, test := range tests {
		keyPath := filepath.Join(tmpDir, test.keyName)
		if err := os.WriteFile(keyPath, test.keyBytes, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	data := []byte("DATA")
	notData := []byte("NOT DATA")

	// Run tests
	for _, test := range tests {
		t.Run(test.keyName, func(t *testing.T) {
			if strings.Contains(test.keyName, "_enc") {
				if runtime.GOOS == "windows" {
					t.Skip("TODO: test encrypted keys on windows")
				}
				t.Setenv("SSH_ASKPASS", scriptPath)
				t.Setenv("SSH_ASKPASS_REQUIRE", "force")
			}

			keyPath := filepath.Join(tmpDir, test.keyName)

			key, err := Import(keyPath)
			if err != nil {
				t.Fatalf("%s: %v", test.keyName, err)
			}
			assert.Equal(t,
				key.keyID,
				test.keyID,
			)

			signer := Signer{
				Key:  key,
				Path: keyPath,
			}

			sig, err := signer.Sign(context.Background(), data)
			if err != nil {
				t.Fatalf("%s: %v", test.keyName, err)
			}

			err = key.Verify(context.Background(), data, sig)
			if err != nil {
				t.Fatalf("%s: %v", test.keyName, err)
			}

			err = key.Verify(context.Background(), notData, sig)
			if err == nil {
				t.Fatalf("%s: %v", test.keyName, err)
			}
		})
	}
}

// Test parseSSH2Key helper function (rsa only)
func TestParseSSH2Key(t *testing.T) {
	data := `---- BEGIN SSH2 PUBLIC KEY ----
Comment: "3072-bit RSA, converted by me@me.me from OpenSSH"
AAAAB3NzaC1yc2EAAAADAQABAAABgQDEI4rdCY/zA3oOMet1JYJ+VugUapNfj7hcAZem1C
Rusd5FTiWVmNh4yywgA+1JWDsBnyLfbOZBiz4fiQQ++bRF/mDXQx2Qr2xgCS27tNyyv8tf
ERGuglAu69T7aLsfPGn4WCaVX3+OuALZVaQl/F5MzoDkiaZkCsBrVZkfL3393Zlhseb/bY
87f7UOwArq3WMMK9Qp0cO8/8rsZnzu3nFClYSILKUx7Vrf7uSaUtl39Dh/QMX1m6Ax0Mh4
3gMnk+Fbrhai+BWo3Y58A5+LBUL3jqDkmXzFvhYJgGKISU5nfKCHDDqlug+l5wJmGus1G8
jZ5uY7s2ZHS5yumPQNoCIZztmLm0DgQqNN4J+Yub5+L6yCgA1Q6mKq/631/DyHvF8e5Gln
COb1zE7zaJacJ42tNdVq7Z3x+Hik9PRfgBPt1oF41SFSCp0YRPLxLMFdTjNgV3HZXVNlq6
6IhyoDZ2hjd5XmMmq7h1a8IybBsItJ8Ikk4X12vIzCSqOlylZS4+U=
---- END SSH2 PUBLIC KEY ----`

	key, err := parseSSH2Key(data)
	if err != nil {
		t.Fatalf("%v", err)
	}
	assert.Equal(t, key.Type(), "ssh-rsa")
}
