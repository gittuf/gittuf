package policy

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"

	"github.com/adityasaky/gittuf/internal/common"
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

func ApplyStagedPolicy() error {
	repo, err := common.GetRepositoryHandler()
	if err != nil {
		return err
	}

	rootPublicKeys, metadata, err := loadCurrentPolicyObjects(repo, PolicyStagingRef)
	if err != nil {
		return err
	}

	// TODO: verify reachable metadata's validity

	// TODO: create RSL entry, this entire operation must be atomic

	return writePolicyObjects(repo, PolicyRef, rootPublicKeys, metadata)
}

func GenerateOrAppendToUnsignedRootMetadata(rootPublicKey tuf.Key, rootThreshold int, expires string, targetsPublicKeys map[string]tuf.Key, targetsThreshold int) (*dsse.Envelope, error) {
	// In this phase, each root key holder adds their keys to the root metadata.
	// They DO NOT sign the root role, and an RSL entry is not created for the
	// policy namespace.
	if err := InitializeNamespace(); err != nil {
		return &dsse.Envelope{}, err
	}

	repo, err := common.GetRepositoryHandler()
	if err != nil {
		return &dsse.Envelope{}, err
	}

	rootMetadata, err := generateOrLoadUnsignedRootMetadata(repo, rootThreshold, expires, targetsPublicKeys, targetsThreshold)
	if err != nil {
		return &dsse.Envelope{}, err
	}

	rootMetadata.Keys[rootPublicKey.ID()] = rootPublicKey
	rootRole := rootMetadata.Roles["root"]
	rootRole.KeyIDs = append(rootRole.KeyIDs, rootPublicKey.ID())
	rootRole.KeyIDs = sortedSet(rootRole.KeyIDs)
	rootRole.Threshold = rootThreshold
	rootMetadata.Roles["root"] = rootRole

	rootMetadataBytes, err := json.Marshal(rootMetadata)
	if err != nil {
		return &dsse.Envelope{}, err
	}

	return &dsse.Envelope{
		PayloadType: RootMetadataPayloadType,
		Payload:     base64.StdEncoding.EncodeToString(rootMetadataBytes),
		Signatures:  []dsse.Signature{},
	}, nil
}

func generateOrLoadUnsignedRootMetadata(repo *git.Repository, rootThreshold int, expires string, targetsPublicKeys map[string]tuf.Key, targetsThreshold int) (*tuf.RootMetadata, error) {
	rootPublicKeys, metadata, err := loadCurrentPolicyObjects(repo, PolicyStagingRef)
	if err != nil {
		if errors.Is(err, ErrNoPolicyExists) {
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

			targetsRole := rootMetadata.Roles["targets"]
			for keyID, key := range targetsPublicKeys {
				rootMetadata.Keys[keyID] = key
				targetsRole.KeyIDs = append(targetsRole.KeyIDs, keyID)
			}
			targetsRole.KeyIDs = sortedSet(targetsRole.KeyIDs)
			rootMetadata.Roles["targets"] = targetsRole

			return rootMetadata, nil
		} else {
			return &tuf.RootMetadata{}, err
		}
	}

	env := metadata[RootMetadataBlobName]

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

	for _, keyID := range rootMetadata.Roles["root"].KeyIDs {
		keyInMetadata := rootMetadata.Keys[keyID]
		keyStored := rootPublicKeys[keyID]
		if keyInMetadata.Scheme != keyStored.Scheme {
			return &tuf.RootMetadata{}, ErrRootKeysDoNotMatch
		}
		if keyInMetadata.KeyType != keyStored.KeyType {
			return &tuf.RootMetadata{}, ErrRootKeysDoNotMatch
		}
		if keyInMetadata.KeyVal != keyStored.KeyVal {
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

func StageUnsignedRootMetadata(rootPublicKey tuf.Key, rootMetadataEnv *dsse.Envelope) error {
	repo, err := common.GetRepositoryHandler()
	if err != nil {
		return err
	}

	if err := writeSingleKey(repo, PolicyStagingRef, rootPublicKey); err != nil {
		return err
	}

	return writeSingleMetadata(repo, PolicyStagingRef, rootMetadataEnv)
}

func LoadStagedUnsignedRootMetadata() (*dsse.Envelope, error) {
	repo, err := common.GetRepositoryHandler()
	if err != nil {
		return &dsse.Envelope{}, err
	}

	_, metadata, err := loadCurrentPolicyObjects(repo, PolicyStagingRef)
	if err != nil {
		return &dsse.Envelope{}, err
	}

	return metadata[RootMetadataBlobName], nil
}

func StageSignedRootMetadata(rootMetadataEnvelope *dsse.Envelope) error {
	repo, err := common.GetRepositoryHandler()
	if err != nil {
		return err
	}

	return writeSingleMetadata(repo, PolicyStagingRef, rootMetadataEnvelope)
}

func sortedSet(list []string) []string {
	m := map[string]bool{}
	for _, l := range list {
		m[l] = true
	}
	keys := maps.Keys(m)
	sort.Slice(keys, func(i int, j int) bool {
		return keys[i] < keys[j]
	})
	return keys
}
