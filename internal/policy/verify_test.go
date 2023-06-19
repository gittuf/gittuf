package policy

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/adityasaky/gittuf/internal/gitinterface"
	"github.com/adityasaky/gittuf/internal/rsl"
	"github.com/adityasaky/gittuf/internal/signerverifier"
	"github.com/adityasaky/gittuf/internal/signerverifier/dsse"
	"github.com/adityasaky/gittuf/internal/tuf"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
)

func TestVerifyEntry(t *testing.T) {
	repo, state := createTestRepository(t, createTestStateWithPolicy)

	entry := rsl.NewEntry("refs/heads/main", plumbing.ZeroHash)
	entryID := createTestRSLEntryCommit(t, repo, entry)
	entry.ID = entryID

	err := verifyEntry(context.Background(), repo, state, entry)
	assert.Nil(t, err)
}

func createTestRSLEntryCommit(t *testing.T, repo *git.Repository, entry *rsl.Entry) plumbing.Hash {
	t.Helper()

	lines := []string{
		rsl.EntryHeader,
		"",
		fmt.Sprintf("%s: %s", rsl.RefKey, entry.RefName),
		fmt.Sprintf("%s: %s", rsl.CommitIDKey, entry.CommitID.String()),
	}

	commitMessage := strings.Join(lines, "\n")

	ref, err := repo.Reference(plumbing.ReferenceName(rsl.RSLRef), true)
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
		Message:      commitMessage,
		TreeHash:     plumbing.ZeroHash,
		ParentHashes: []plumbing.Hash{ref.Hash()},
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

	if err := gitinterface.ApplyCommit(repo, testCommit, ref); err != nil {
		t.Fatal(err)
	}

	ref, err = repo.Reference(plumbing.ReferenceName(rsl.RSLRef), true)
	if err != nil {
		t.Fatal(err)
	}

	return ref.Hash()
}

func createTestStateWithPolicy(t *testing.T) *State {
	t.Helper()

	signingKeyBytes, err := os.ReadFile(filepath.Join("test-data", "signing-key"))
	if err != nil {
		t.Fatal(err)
	}
	signer, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(signingKeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	keyBytes, err := os.ReadFile(filepath.Join("test-data", "targets-2.pub"))
	if err != nil {
		t.Fatal(err)
	}
	key, err := tuf.LoadKeyFromBytes(keyBytes)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err := InitializeRootMetadata(key)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err = AddTargetsKey(rootMetadata, key)
	if err != nil {
		t.Fatal(err)
	}

	rootEnv, err := dsse.CreateEnvelope(rootMetadata)
	if err != nil {
		t.Fatal(err)
	}
	rootEnv, err = dsse.SignEnvelope(context.Background(), rootEnv, signer)
	if err != nil {
		t.Fatal(err)
	}

	gpgKeyBytes, err := os.ReadFile(filepath.Join("test-data", "gpg-pubkey.asc"))
	if err != nil {
		t.Fatal(err)
	}
	gpgKey := &tuf.Key{
		KeyType: "gpg",
		Scheme:  "gpg",
		KeyVal: tuf.KeyVal{
			Public: strings.TrimSpace(string(gpgKeyBytes)),
		},
	}
	gpgKey.ID() //nolint:errcheck

	targetsMetadata := InitializeTargetsMetadata()
	targetsMetadata, err = AddOrUpdateDelegation(targetsMetadata, "protect-main", []*tuf.Key{gpgKey}, []string{"git:refs/heads/main"})
	if err != nil {
		t.Fatal(err)
	}

	targetsEnv, err := dsse.CreateEnvelope(targetsMetadata)
	if err != nil {
		t.Fatal(err)
	}
	targetsEnv, err = dsse.SignEnvelope(context.Background(), targetsEnv, signer)
	if err != nil {
		t.Fatal(err)
	}

	return &State{
		RootEnvelope:    rootEnv,
		TargetsEnvelope: targetsEnv,
		RootPublicKeys:  []*tuf.Key{key},
		AllPublicKeys:   []*tuf.Key{key, gpgKey},
	}
}
