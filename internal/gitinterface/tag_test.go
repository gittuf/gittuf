// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/gittuf/gittuf/internal/signerverifier"
	"github.com/gittuf/gittuf/internal/signerverifier/gpg"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
	sslibsv "github.com/secure-systems-lab/go-securesystemslib/signerverifier"
	"github.com/stretchr/testify/assert"
)

func TestTag(t *testing.T) {
	repo, err := git.Init(memory.NewStorage(), memfs.New())
	if err != nil {
		t.Fatal(err)
	}

	refName := "refs/heads/main"
	tagName := "v0.1.0"
	clock = testClock
	getGitConfig = func(repo *git.Repository) (*config.Config, error) {
		return testGitConfig, nil
	}

	// Try to create tag with an unknown underlying object
	_, err = Tag(repo, plumbing.ZeroHash, tagName, tagName, false)
	assert.ErrorIs(t, err, plumbing.ErrObjectNotFound)

	// Create a commit and retry
	emptyTreeHash, err := WriteTree(repo, nil)
	if err != nil {
		t.Fatal(err)
	}
	commitID, err := Commit(repo, emptyTreeHash, refName, "Initial commit", false)
	if err != nil {
		t.Fatal(err)
	}

	tagHash, err := Tag(repo, commitID, tagName, tagName, false)
	assert.Nil(t, err)
	assert.Equal(t, "8b195348588d8a48060ec8d5436459b825a1b352", tagHash.String())

	tag, err := repo.TagObject(tagHash)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, tagName, tag.Name)
	assert.Equal(t, plumbing.CommitObject, tag.TargetType)

	// Check tag reference is set correctly
	ref, err := repo.Tag(tagName)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, tagHash, ref.Hash())

	// Try to create a tag with the same name, expect error
	_, err = Tag(repo, commitID, tagName, tagName, false)
	assert.ErrorIs(t, err, ErrTagAlreadyExists)
}

func TestVerifyTagSignature(t *testing.T) {
	gpgSignedTag := createTestSignedTag(t)

	keyBytes, err := os.ReadFile(filepath.Join("test-data", "gpg-pubkey.asc"))
	if err != nil {
		t.Fatal(err)
	}

	gpgKey, err := gpg.LoadGPGKeyFromBytes(keyBytes)
	if err != nil {
		t.Fatal(err)
	}

	fulcioKey := &sslibsv.SSLibKey{
		KeyType: signerverifier.FulcioKeyType,
		Scheme:  "fulcio",
		KeyVal: sslibsv.KeyVal{
			Identity: testEmail,
			Issuer:   "https://github.com/login/oauth",
		},
	}

	t.Run("gpg signed tag with correct gpg key", func(t *testing.T) {
		err = VerifyTagSignature(context.Background(), gpgSignedTag, gpgKey)
		assert.Nil(t, err)
	})

	t.Run("gpg signed tag with gitsign identity", func(t *testing.T) {
		err = VerifyTagSignature(context.Background(), gpgSignedTag, fulcioKey)
		assert.ErrorIs(t, err, ErrIncorrectVerificationKey)
	})
}

func createTestSignedTag(t *testing.T) *object.Tag {
	t.Helper()

	repo, err := git.Init(memory.NewStorage(), memfs.New())
	if err != nil {
		t.Fatal(err)
	}

	testTag := &object.Tag{
		Name:    "v1",
		Message: "v1",
		Tagger: object.Signature{
			Name:  testName,
			Email: testEmail,
			When:  testClock.Now(),
		},
		TargetType: plumbing.CommitObject,
		Target:     plumbing.ZeroHash,
	}

	tagEncoded := repo.Storer.NewEncodedObject()
	if err := testTag.EncodeWithoutSignature(tagEncoded); err != nil {
		t.Fatal(err)
	}
	r, err := tagEncoded.Reader()
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
	testTag.PGPSignature = sig.String()

	return testTag
}
