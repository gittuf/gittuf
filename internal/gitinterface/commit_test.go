package gitinterface

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/adityasaky/gittuf/internal/tuf"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	format "github.com/go-git/go-git/v5/plumbing/format/config"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
)

func TestCreateCommitObject(t *testing.T) {
	gitConfig := &config.Config{
		Raw: &format.Config{
			Sections: format.Sections{
				&format.Section{
					Name: "user",
					Options: format.Options{
						&format.Option{
							Key:   "name",
							Value: "Jane Doe",
						},
						&format.Option{
							Key:   "email",
							Value: "jane.doe@example.com",
						},
					},
				},
			},
		},
	}

	clock := clockwork.NewFakeClockAt(time.Date(1995, time.October, 26, 9, 0, 0, 0, time.UTC))
	commit := createCommitObject(gitConfig, plumbing.ZeroHash, plumbing.ZeroHash, "Test commit", clock)

	enc := memory.NewStorage().NewEncodedObject()
	if err := commit.Encode(enc); err != nil {
		t.Error(err)
	}

	assert.Equal(t, plumbing.NewHash("dce09cc0f41eaa323f6949142d66ead789f40f6f"), enc.Hash())
}

func TestVerifyCommitSignature(t *testing.T) {
	testCommit := createTestSignedCommit(t)

	keyBytes, err := os.ReadFile(filepath.Join("test-data", "gpg-pubkey.asc"))
	if err != nil {
		t.Fatal(err)
	}

	key := &tuf.Key{
		KeyType: "gpg",
		Scheme:  "gpg",
		KeyVal: tuf.KeyVal{
			Public: strings.TrimSpace(string(keyBytes)),
		},
	}

	err = VerifyCommitSignature(testCommit, key)
	assert.Nil(t, err)
}

func createTestSignedCommit(t *testing.T) *object.Commit {
	t.Helper()

	repo, err := git.Init(memory.NewStorage(), memfs.New())
	if err != nil {
		t.Fatal(err)
	}

	clock := clockwork.NewFakeClockAt(time.Date(1995, time.October, 26, 9, 0, 0, 0, time.UTC))
	testCommit := &object.Commit{
		Author: object.Signature{
			Name:  "Jane Doe",
			Email: "jane.doe@example.com",
			When:  clock.Now(),
		},
		Committer: object.Signature{
			Name:  "Jane Doe",
			Email: "jane.doe@example.com",
			When:  clock.Now(),
		},
		Message:  "Test commit",
		TreeHash: plumbing.ZeroHash,
	}

	commitEncoded := repo.Storer.NewEncodedObject()
	if err := testCommit.EncodeWithoutSignature(commitEncoded); err != nil {
		t.Fatal(err)
	}
	r, err := commitEncoded.Reader()
	if err != nil {
		t.Fatal(err)
	}

	signingKeyBytes, err := os.ReadFile(filepath.Join("test-data", "gpg-privkey.asc"))
	if err != nil {
		t.Fatal(err)
	}

	keyring, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(signingKeyBytes))
	if err != nil {
		t.Fatal(err)
	}

	sig := new(strings.Builder)
	if err := openpgp.ArmoredDetachSign(sig, keyring[0], r, nil); err != nil {
		t.Fatal(err)
	}
	testCommit.PGPSignature = sig.String()

	return testCommit
}
