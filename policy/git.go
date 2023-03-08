package policy

// This file has a bunch of APIs to work with the custom format of the policy namespace.

import (
	"encoding/json"
	"errors"

	"github.com/adityasaky/gittuf/internal/gitinterface"
	"github.com/adityasaky/gittuf/pkg/tuf"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
)

var ErrNoPolicyExists = errors.New("policy state does not exist")

const (
	RootPublicKeysTreeEntryName = "keys"
	MetadataTreeEntryName       = "metadata"
)

func loadCurrentPolicyObjects(repo *git.Repository, refName string) (map[string]tuf.Key, map[string]*dsse.Envelope, error) {
	ref, err := repo.Reference(plumbing.ReferenceName(refName), true)
	if err != nil {
		return map[string]tuf.Key{}, map[string]*dsse.Envelope{}, err
	}

	if ref.Hash().IsZero() {
		return map[string]tuf.Key{}, map[string]*dsse.Envelope{}, ErrNoPolicyExists
	}

	tipCommit, err := repo.CommitObject(ref.Hash())
	if err != nil {
		return map[string]tuf.Key{}, map[string]*dsse.Envelope{}, err
	}

	tipTree, err := repo.TreeObject(tipCommit.TreeHash)
	if err != nil {
		return map[string]tuf.Key{}, map[string]*dsse.Envelope{}, err
	}

	keysEntry, err := tipTree.FindEntry(RootPublicKeysTreeEntryName)
	if err != nil {
		return map[string]tuf.Key{}, map[string]*dsse.Envelope{}, err
	}

	rootPublicKeys, err := loadCurrentKeys(repo, keysEntry.Hash)
	if err != nil {
		return map[string]tuf.Key{}, map[string]*dsse.Envelope{}, err
	}

	metadataEntry, err := tipTree.FindEntry(MetadataTreeEntryName)
	if err != nil {
		return map[string]tuf.Key{}, map[string]*dsse.Envelope{}, err
	}

	metadata, err := loadCurrentMetadata(repo, metadataEntry.Hash)
	if err != nil {
		return map[string]tuf.Key{}, map[string]*dsse.Envelope{}, err
	}

	return rootPublicKeys, metadata, nil
}

func loadCurrentKeys(repo *git.Repository, treeHash plumbing.Hash) (map[string]tuf.Key, error) {
	treeObj, err := repo.TreeObject(treeHash)
	if err != nil {
		return map[string]tuf.Key{}, err
	}

	rootPublicKeys := map[string]tuf.Key{}
	for _, e := range treeObj.Entries {
		_, keyContents, err := gitinterface.ReadBlob(repo, e.Hash)
		if err != nil {
			return map[string]tuf.Key{}, err
		}
		key, err := tuf.LoadKeyFromBytes(keyContents)
		if err != nil {
			return map[string]tuf.Key{}, err
		}
		rootPublicKeys[e.Name] = key
	}

	return rootPublicKeys, nil
}

func loadCurrentMetadata(repo *git.Repository, treeHash plumbing.Hash) (map[string]*dsse.Envelope, error) {
	treeObj, err := repo.TreeObject(treeHash)
	if err != nil {
		return map[string]*dsse.Envelope{}, err
	}

	metadata := map[string]*dsse.Envelope{}
	for _, e := range treeObj.Entries {
		_, metadataContents, err := gitinterface.ReadBlob(repo, e.Hash)
		if err != nil {
			return map[string]*dsse.Envelope{}, err
		}
		var env dsse.Envelope
		if err := json.Unmarshal(metadataContents, &env); err != nil {
			return map[string]*dsse.Envelope{}, err
		}
		metadata[e.Name] = &env
	}
	return metadata, nil
}

func writePolicyObjects(repo *git.Repository, refName string, rootPublicKeys map[string]tuf.Key, metadata map[string]*dsse.Envelope) error {
	newTree, err := writeParentTree(repo, rootPublicKeys, metadata)
	if err != nil {
		return err
	}

	return gitinterface.Commit(repo, newTree, refName, "Writing new policy objects\n", true)
}

func writeSingleKey(repo *git.Repository, refName string, newKey tuf.Key) error {
	rootPublicKeys := map[string]tuf.Key{}
	metadata := map[string]*dsse.Envelope{}

	rootPublicKeys, metadata, err := loadCurrentPolicyObjects(repo, refName)
	if err != nil && !errors.Is(err, ErrNoPolicyExists) {
		return err
	}

	rootPublicKeys[newKey.ID()] = newKey
	return writePolicyObjects(repo, refName, rootPublicKeys, metadata)
}

func writeSingleMetadata(repo *git.Repository, refName string, newMetadata *dsse.Envelope) error {
	rootPublicKeys := map[string]tuf.Key{}
	metadata := map[string]*dsse.Envelope{}

	rootPublicKeys, metadata, err := loadCurrentPolicyObjects(repo, refName)
	if err != nil && !errors.Is(err, ErrNoPolicyExists) {
		return err
	}

	switch newMetadata.PayloadType {
	case RootMetadataPayloadType:
		metadata[RootMetadataBlobName] = newMetadata
	case TargetsMetadataPayloadType:
		metadata[TargetsMetadataBlobName] = newMetadata
	}

	return writePolicyObjects(repo, refName, rootPublicKeys, metadata)
}

func writeParentTree(repo *git.Repository, rootPublicKeys map[string]tuf.Key, metadata map[string]*dsse.Envelope) (plumbing.Hash, error) {
	entries := []object.TreeEntry{}

	keysTreeID, err := writeKeysTree(repo, rootPublicKeys)
	if err != nil {
		return plumbing.ZeroHash, err
	}
	entries = append(entries, object.TreeEntry{
		Name: RootPublicKeysTreeEntryName,
		Hash: keysTreeID,
		Mode: filemode.Dir,
	})

	metadataTreeID, err := writeMetadataTree(repo, metadata)
	if err != nil {
		return plumbing.ZeroHash, err
	}
	entries = append(entries, object.TreeEntry{
		Name: MetadataTreeEntryName,
		Hash: metadataTreeID,
		Mode: filemode.Dir,
	})

	return gitinterface.WriteTree(repo, entries)
}

func writeKeysTree(repo *git.Repository, rootPublicKeys map[string]tuf.Key) (plumbing.Hash, error) {
	entries := []object.TreeEntry{}
	for keyID, key := range rootPublicKeys {
		keyContents, err := json.Marshal(&key)
		if err != nil {
			return plumbing.ZeroHash, err
		}
		_, keyHash, err := gitinterface.WriteBlob(repo, keyContents)
		if err != nil {
			return plumbing.ZeroHash, err
		}

		entries = append(entries, object.TreeEntry{
			Name: keyID,
			Hash: keyHash,
			Mode: filemode.Regular,
		})
	}
	return gitinterface.WriteTree(repo, entries)
}

func writeMetadataTree(repo *git.Repository, items map[string]*dsse.Envelope) (plumbing.Hash, error) {
	entries := []object.TreeEntry{}
	for name, env := range items {
		contents, err := json.Marshal(&env)
		if err != nil {
			return plumbing.ZeroHash, err
		}
		_, metadataID, err := gitinterface.WriteBlob(repo, contents)
		if err != nil {
			return plumbing.ZeroHash, err
		}

		entries = append(entries, object.TreeEntry{
			Name: name,
			Hash: metadataID,
			Mode: filemode.Regular,
		})
	}

	return gitinterface.WriteTree(repo, entries)
}
