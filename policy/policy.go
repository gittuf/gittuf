package policy

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"time"

	"github.com/adityasaky/gittuf/internal/common"
	"github.com/adityasaky/gittuf/internal/gitinterface"
	"github.com/adityasaky/gittuf/pkg/tuf"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	"golang.org/x/exp/maps"
)

var (
	ErrUnexpectedPayloadType          = errors.New("unexpected payload type encountered")
	ErrUnsignedMetadataHasSignatures  = errors.New("metadata expected to be unsigned has signatures")
	ErrMisalignedRootPublicKeyEntries = errors.New("number of keys expected for root role differs from keys stored as blobs")
	ErrRootKeysDoNotMatch             = errors.New("root key in root metadata differs from key stored with same ID") // this is really redundant because if everything is implemented correctly, this error would indicate a hash collision
	ErrMisalignedRootExpectations     = errors.New("existing unsigned root metadata differs from expectations defined during this ceremony")
)

const (
	PolicyRef                  = "refs/gittuf/policy"
	PolicyStagingRef           = "refs/gittuf/policy-staging"
	RootMetadataBlobName       = "root.json"
	TargetsMetadataBlobName    = "targets.json"
	RootMetadataPayloadType    = "application/vnd.gittuf-root+json"
	TargetsMetadataPayloadType = "application/vnd.gittuf-targets+json"
)

// InitializeNamespace creates a git ref for the policy. Initially, the entry
// has a zero hash.
// Note: policy.InitializeNamespace assumes the gittuf namespace has been
// created already.
func InitializeNamespace() error {
	repoRootDir, err := common.GetRepositoryRootDirectory()
	if err != nil {
		return err
	}

	refPaths := []string{
		filepath.Join(repoRootDir, common.GetGitDir(), PolicyRef),
		filepath.Join(repoRootDir, common.GetGitDir(), PolicyStagingRef),
	}
	for _, refPath := range refPaths {
		if _, err := os.Stat(refPath); err != nil {
			if os.IsNotExist(err) {
				if err := os.WriteFile(refPath, plumbing.ZeroHash[:], 0644); err != nil {
					return err
				}
			} else {
				return err
			}
		}
	}

	return nil
}

func GenerateOrAppendToUnsignedRootMetadata(rootPublicKey tuf.Key, rootThreshold int, expires time.Time, targetsPublicKeys map[string]tuf.Key, targetsThreshold int) error {
	// In this phase, each root key holder adds their keys to the root metadata.
	// They DO NOT sign the root role, and an RSL entry is not created for the
	// policy namespace.
	if err := InitializeNamespace(); err != nil {
		return err
	}

	repo, err := common.GetRepositoryHandler()
	if err != nil {
		return err
	}

	rootMetadata, err := generateOrLoadUnsignedRootMetadata(repo, rootThreshold, expires, targetsPublicKeys, targetsThreshold)
	if err != nil {
		return err
	}

	rootMetadata.Keys[rootPublicKey.ID()] = rootPublicKey
	rootRole := rootMetadata.Roles["root"]
	rootRole.KeyIDs = append(rootRole.KeyIDs, rootPublicKey.ID())
	rootRole.Threshold = rootThreshold
	rootMetadata.Roles["root"] = rootRole

	return nil
}

func generateOrLoadUnsignedRootMetadata(repo *git.Repository, rootThreshold int, expires time.Time, targetsPublicKeys map[string]tuf.Key, targetsThreshold int) (*tuf.RootMetadata, error) {
	ref, err := repo.Reference(plumbing.ReferenceName(PolicyStagingRef), true)
	if err != nil {
		return &tuf.RootMetadata{}, err
	}

	if !ref.Hash().IsZero() {
		rootMetadata := &tuf.RootMetadata{
			Type:               "root",
			SpecVersion:        "1.0",
			ConsistentSnapshot: true,
			Version:            1,
			Expires:            expires,
			Keys:               map[string]tuf.Key{},
			Roles: map[string]tuf.Role{
				"root": {
					KeyIDs: []string{},
				},
				"targets": {
					Threshold: targetsThreshold,
				},
			},
		}

		for keyID, key := range targetsPublicKeys {
			rootMetadata.Keys[keyID] = key
			targetsRole := rootMetadata.Roles["targets"]
			targetsRole.KeyIDs = append(targetsRole.KeyIDs, keyID)
			rootMetadata.Roles["targets"] = targetsRole
		}
		return rootMetadata, nil
	}

	tipCommit, err := repo.CommitObject(ref.Hash())
	if err != nil {
		return &tuf.RootMetadata{}, err
	}

	tipTree, err := repo.TreeObject(tipCommit.TreeHash)
	if err != nil {
		return &tuf.RootMetadata{}, err
	}

	var metadataTreeID plumbing.Hash
	var rootPublicKeysTreeID plumbing.Hash
	for _, e := range tipTree.Entries {
		switch e.Name {
		case "metadata":
			metadataTreeID = e.Hash
		case "keys":
			rootPublicKeysTreeID = e.Hash
		}
	}

	metadataTree, err := repo.TreeObject(metadataTreeID)
	if err != nil {
		return &tuf.RootMetadata{}, err
	}

	var rootMetadataBlobID plumbing.Hash
	for _, e := range metadataTree.Entries {
		if e.Name == RootMetadataBlobName {
			rootMetadataBlobID = e.Hash
			break
		}
	}

	_, contents, err := gitinterface.ReadBlob(repo, rootMetadataBlobID)
	if err != nil {
		return &tuf.RootMetadata{}, err
	}

	env := &dsse.Envelope{}
	if err := json.Unmarshal(contents, env); err != nil {
		return &tuf.RootMetadata{}, err
	}

	// Validate what we loaded
	if env.PayloadType != RootMetadataPayloadType {
		return &tuf.RootMetadata{}, ErrUnexpectedPayloadType
	}
	if len(env.Signatures) != 0 {
		return &tuf.RootMetadata{}, ErrUnsignedMetadataHasSignatures
	}

	payload, err := env.DecodeB64Payload()
	if err != nil {
		return &tuf.RootMetadata{}, err
	}
	var rootMetadata tuf.RootMetadata
	if err := json.Unmarshal(payload, &rootMetadata); err != nil {
		return &tuf.RootMetadata{}, err
	}

	// We finally have the staged root metadata file but we're not done yet.
	rootPublicKeysTree, err := repo.TreeObject(rootPublicKeysTreeID)
	if err != nil {
		return &tuf.RootMetadata{}, err
	}

	if len(rootPublicKeysTree.Entries) != len(rootMetadata.Roles["root"].KeyIDs) {
		return &tuf.RootMetadata{}, ErrMisalignedRootPublicKeyEntries
	}

	storedRootPublicKeys := map[string]tuf.Key{}
	for _, e := range rootPublicKeysTree.Entries {
		if _, keyContents, err := gitinterface.ReadBlob(repo, e.Hash); err != nil {
			return &tuf.RootMetadata{}, err
		} else {
			if key, err := tuf.LoadKeyFromBytes(keyContents); err != nil {
				return &tuf.RootMetadata{}, err
			} else {
				storedRootPublicKeys[key.ID()] = key
			}
		}
	}

	for _, keyID := range rootMetadata.Roles["root"].KeyIDs {
		keyInMetadata := rootMetadata.Keys[keyID]
		keyStored := storedRootPublicKeys[keyID]
		if keyInMetadata != keyStored {
			return &tuf.RootMetadata{}, ErrRootKeysDoNotMatch
		}
	}

	if rootMetadata.Expires != expires {
		return &tuf.RootMetadata{}, ErrMisalignedRootExpectations
	}

	if rootMetadata.Roles["root"].Threshold != rootThreshold {
		return &tuf.RootMetadata{}, ErrMisalignedRootExpectations
	}

	// TODO: Instead of expecting alignment, we can create a set of targets keys
	// if different root key holders are providing different targets pubkeys.
	targetsPublicKeyIDs := maps.Keys(targetsPublicKeys)
	targetsRole := rootMetadata.Roles["targets"]
	sort.Strings(targetsRole.KeyIDs)
	sort.Strings(targetsPublicKeyIDs)
	if !reflect.DeepEqual(targetsRole.KeyIDs, targetsPublicKeyIDs) {
		return &tuf.RootMetadata{}, ErrMisalignedRootExpectations
	}

	if rootMetadata.Roles["targets"].Threshold != targetsThreshold {
		return &tuf.RootMetadata{}, ErrMisalignedRootExpectations
	}

	return &rootMetadata, nil
}
