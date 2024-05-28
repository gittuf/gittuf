package ssh

import (
	"context"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gittuf/gittuf/internal/common"
	"github.com/stretchr/testify/assert"
)

// TODO: Is there a more idomatic way to do this in go tests?
func testDataPath(name string) string {
	dir, _ := filepath.Abs("../../testartifacts/testdata/")
	return path.Join(dir, name)
}

// Basic smoke test for ssh package for all supported keys
func TestSSH(t *testing.T) {
	testSSHKeys := common.TestSSHKeys(t)

	rsa_keyid := "SHA256:ESJezAOo+BsiEpddzRXS6+wtF16FID4NCd+3gj96rFo"
	ecdsa_keyid := "SHA256:oNYBImx035m3rl1Sn/+j5DPrlS9+zXn7k3mjNrC5eto"
	ed25519_keyid := "SHA256:cewFulOIcROWnolPTGEQXG4q7xvLIn3kNTCMqdfoP4E"

	tests := []struct {
		keyName string
		keyID   string
	}{
		{"rsa", rsa_keyid},
		{"rsa_enc", rsa_keyid},
		{"rsa.pub", rsa_keyid},
		{"ecdsa", ecdsa_keyid},
		{"ecdsa_enc", ecdsa_keyid},
		{"ecdsa.pub", ecdsa_keyid},
		{"ed25519", ed25519_keyid},
		{"ed25519_enc", ed25519_keyid},
		{"ed25519.pub", ed25519_keyid},
	}

	for _, test := range tests {
		t.Run(test.keyName, func(t *testing.T) {
			if strings.Contains(test.keyName, "_enc") {
				t.Setenv("SSH_ASKPASS", testDataPath("scripts/askpass.sh"))
				t.Setenv("SSH_ASKPASS_REQUIRE", "force")
			}

			path := path.Join(testSSHKeys, test.keyName)
			key, err := Import(path)
			if err != nil {
				t.Fatalf("%s: %v", test.keyName, err)
			}
			assert.Equal(t,
				key.keyID,
				test.keyID,
			)

			signer := Signer{
				Key:  key,
				Path: path,
			}

			data := []byte("DATA")
			sig, err := signer.Sign(context.TODO(), data)
			if err != nil {
				t.Fatalf("%s: %v", test.keyName, err)
			}

			err = key.Verify(context.TODO(), data, sig)
			if err != nil {
				t.Fatalf("%s: %v", test.keyName, err)
			}

			err = key.Verify(context.TODO(), []byte("NOT DATA"), sig)
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
