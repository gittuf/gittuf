package policy

import (
	"errors"
	"os"
	"path/filepath"
	"sort"

	"github.com/adityasaky/gittuf/internal/common"
	"github.com/go-git/go-git/v5/plumbing"
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
