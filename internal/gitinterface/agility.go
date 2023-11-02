package gitinterface

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

const HashAgilityRef = "refs/gittuf/hash-agility"

var ErrHashCollisionDetected = errors.New("hash collision detected")

// RecordHashEntry records the alternative hash for the specified object using a
// stronger algorithm than SHA-1.
// Experimental: This function is not intended for use in user workflows yet.
func RecordHashEntry(repo *git.Repository, sha1 plumbing.Hash, hashAlg HashAlg) error {
	ref, err := repo.Reference(HashAgilityRef, true)
	if err != nil {
		if errors.Is(err, plumbing.ErrReferenceNotFound) {
			if err := repo.Storer.SetReference(plumbing.NewHashReference(HashAgilityRef, plumbing.ZeroHash)); err != nil {
				return err
			}
			ref, err = repo.Reference(HashAgilityRef, true)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	hashMapping := map[string]string{}
	exists := true
	contents, err := ReadBlob(repo, ref.Hash())
	if err != nil {
		if !errors.Is(err, plumbing.ErrObjectNotFound) {
			// mapping doesn't exist yet, return only for other errors
			return err
		}
		exists = false
	}
	if exists {
		if err := json.Unmarshal(contents, &hashMapping); err != nil {
			return err
		}
	}

	_, hashMapping, err = addHashEntry(repo, hashMapping, sha1, hashAlg)
	if err != nil {
		return err
	}

	contents, err = json.Marshal(hashMapping)
	if err != nil {
		return err
	}
	newHashMappingHash, err := WriteBlob(repo, contents)
	if err != nil {
		return err
	}

	newRef := plumbing.NewHashReference(HashAgilityRef, newHashMappingHash)
	return repo.Storer.CheckAndSetReference(newRef, ref)
}

func addHashEntry(repo *git.Repository, hashMapping map[string]string, sha1 plumbing.Hash, hashAlg HashAlg) (Hash, map[string]string, error) {
	obj, err := repo.Storer.EncodedObject(plumbing.AnyObject, sha1)
	if err != nil {
		return SHA256ZeroHash, nil, err
	}

	var sha256Hash Hash
	switch obj.Type() {
	case plumbing.BlobObject:
		reader, err := obj.Reader()
		if err != nil {
			return SHA256ZeroHash, nil, err
		}
		contents, err := io.ReadAll(reader)
		if err != nil {
			return SHA256ZeroHash, nil, err
		}

		sha256Hash = hashObj(plumbing.BlobObject, contents, hashAlg)
	case plumbing.TreeObject:
		tree := &object.Tree{}
		if err := tree.Decode(obj); err != nil {
			return SHA256ZeroHash, nil, err
		}

		var updatedEntryHash Hash
		var contents bytes.Buffer
		for _, entry := range tree.Entries {
			updatedEntryHash, hashMapping, err = addHashEntry(repo, hashMapping, entry.Hash, hashAlg)
			if err != nil {
				return SHA256ZeroHash, nil, err
			}
			if _, err := contents.WriteString(fmt.Sprintf("%o %s", entry.Mode, entry.Name)); err != nil {
				return SHA256ZeroHash, nil, err
			}
			if _, err := contents.Write([]byte{0x00}); err != nil {
				return SHA256ZeroHash, nil, err
			}
			if _, err := contents.Write(updatedEntryHash.Bytes()); err != nil {
				return SHA256ZeroHash, nil, err
			}
		}

		sha256Hash = hashObj(plumbing.TreeObject, contents.Bytes(), hashAlg)
	case plumbing.CommitObject:
		commit := &object.Commit{}
		if err := commit.Decode(obj); err != nil {
			return SHA256ZeroHash, nil, err
		}

		var contents bytes.Buffer
		var updatedTreeHash Hash
		updatedTreeHash, hashMapping, err = addHashEntry(repo, hashMapping, commit.TreeHash, hashAlg)
		if err != nil {
			return SHA256ZeroHash, nil, err
		}
		if _, err := contents.WriteString(fmt.Sprintf("tree %s\n", updatedTreeHash)); err != nil {
			return SHA256ZeroHash, nil, err
		}

		var updatedParentHash Hash
		for _, parentHash := range commit.ParentHashes {
			updatedParentHash, hashMapping, err = addHashEntry(repo, hashMapping, parentHash, hashAlg)
			if err != nil {
				return SHA256ZeroHash, nil, err
			}

			if _, err := contents.WriteString(fmt.Sprintf("parent %s\n", updatedParentHash)); err != nil {
				return SHA256ZeroHash, nil, err
			}
		}

		if _, err := contents.WriteString("author "); err != nil {
			return SHA256ZeroHash, nil, err
		}
		if err := commit.Author.Encode(&contents); err != nil {
			return SHA256ZeroHash, nil, err
		}

		if _, err := contents.WriteString("\ncommitter "); err != nil {
			return SHA256ZeroHash, nil, err
		}
		if err := commit.Committer.Encode(&contents); err != nil {
			return SHA256ZeroHash, nil, err
		}

		if len(commit.PGPSignature) != 0 {
			if _, err := contents.WriteString("\ngpgsig "); err != nil {
				return SHA256ZeroHash, nil, err
			}
		}

		sig := strings.TrimSuffix(commit.PGPSignature, "\n")
		lines := strings.Split(sig, "\n")
		if _, err := contents.WriteString(strings.Join(lines, "\n ")); err != nil {
			return SHA256ZeroHash, nil, err
		}

		if _, err := contents.WriteString(fmt.Sprintf("\n\n%s", commit.Message)); err != nil {
			return SHA256ZeroHash, nil, err
		}

		sha256Hash = hashObj(plumbing.CommitObject, contents.Bytes(), hashAlg)
	case plumbing.TagObject:
		tag := &object.Tag{}
		if err := tag.Decode(obj); err != nil {
			return SHA256ZeroHash, nil, err
		}

		var updatedTargetHash Hash
		updatedTargetHash, hashMapping, err = addHashEntry(repo, hashMapping, tag.Target, hashAlg)
		if err != nil {
			return SHA256ZeroHash, nil, err
		}

		var contents bytes.Buffer
		if _, err := contents.WriteString(fmt.Sprintf("object %s\ntype %s\ntag %s\ntagger ", updatedTargetHash.String(), tag.TargetType.Bytes(), tag.Name)); err != nil {
			return SHA256ZeroHash, nil, err
		}

		if err := tag.Tagger.Encode(&contents); err != nil {
			return SHA256ZeroHash, nil, err
		}

		if _, err := contents.WriteString(fmt.Sprintf("\n\n%s", tag.Message)); err != nil {
			return SHA256ZeroHash, nil, err
		}

		// go-git warns about messages having to end with a newline here to
		// separate the signature. We're not doing any checks because if there
		// was a corruption, we will catch it with the original tag object.

		if len(tag.PGPSignature) != 0 {
			if _, err := contents.WriteString(tag.PGPSignature); err != nil {
				return SHA256ZeroHash, nil, err
			}
		}

		sha256Hash = hashObj(plumbing.TagObject, contents.Bytes(), hashAlg)
	}

	if existing, ok := hashMapping[sha1.String()]; ok {
		if existing != sha256Hash.String() {
			return SHA256ZeroHash, nil, ErrHashCollisionDetected
		}

		return sha256Hash, hashMapping, nil
	}

	hashMapping[sha1.String()] = sha256Hash.String()
	return sha256Hash, hashMapping, nil
}
