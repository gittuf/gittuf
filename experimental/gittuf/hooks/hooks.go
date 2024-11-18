// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package hooks

import (
	"github.com/gittuf/gittuf/internal/rsl"
	sslibdsse "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
	tufv01 "github.com/gittuf/gittuf/internal/tuf/v01"

	"encoding/json"
	"errors"
	"fmt"
	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/tuf"
	"log/slog"
	"path"
	"strings"
)

const (
	// HooksRef defines the Git namespace used for gittuf-enabled hooks
	HooksRef = "refs/gittuf/hooks"

	// TargetsRoleName defines the expected name for the top level gittuf policy file.
	TargetsRoleName = "targets"

	// HooksRoleName defines the expected name for the hooks file.
	HooksRoleName = "hooks"

	// DefaultCommitMessage defines the fallback message to use when adding a new hook if an action specific message is unavailable.
	DefaultCommitMessage = "Create hooks ref"

	ApplyMessage          = "Apply hooks"
	metadataTreeEntryName = "metadata"
	hooksTreeEntryName    = "hooks"
)

var (
	ErrMetadataNotFound          = errors.New("unable to find requested metadata file")
	ErrPolicyNotFound            = errors.New("cannot find policy")
	ErrHooksMetadataHashMismatch = errors.New("error verifying Hooks metadata - hashes do not match")
)

type HookState struct {
	RootEnvelope        *sslibdsse.Envelope
	TargetsEnvelope     *sslibdsse.Envelope
	HooksEnvelope       *sslibdsse.Envelope
	DelegationEnvelopes map[string]*sslibdsse.Envelope
	RootPublicKeys      []tuf.Principal
	Repository          *gitinterface.Repository
}

type Metadata struct {
	HooksInfo map[string]*Hook  `json:"HooksInfo"`
	Bindings  map[string]string `json:"Bindings"`
}

type HookIdentifiers struct {
	Filepath    string
	Stage       string
	Hookname    string
	Environment string
	Modules     []string
}

type Hook struct {
	SHA256Hash  string   `json:"SHA256Hash"`
	BlobID      string   `json:"BlobID"`
	Stage       string   `json:"Stage"`
	Branches    []string `json:"Branches"`
	Environment string   `json:"Environment"`
	Modules     []string `json:"Modules"`
}

type regularSearcher struct {
	repo *gitinterface.Repository
}

func newSearcher(repo *gitinterface.Repository) *regularSearcher {
	return &regularSearcher{repo: repo}
}

// FindHooksEntryFor returns the reference entry based on the ref name (HooksRef) here
// returns *rsl.ReferenceEntry
func (r *regularSearcher) FindHooksEntryFor(entry rsl.Entry) (*rsl.ReferenceEntry, error) {
	// If the requested entry itself is for the policy ref, return as is
	if entry, isReferenceEntry := entry.(*rsl.ReferenceEntry); isReferenceEntry && entry.RefName == HooksRef {
		slog.Debug(fmt.Sprintf("Initial entry '%s' is for gittuf policy, setting that as current policy...", entry.GetID().String()))
		return entry, nil
	}

	policyEntry, _, err := rsl.GetLatestReferenceEntry(r.repo, rsl.ForReference(HooksRef), rsl.BeforeEntryID(entry.GetID()))
	if err != nil {
		if errors.Is(err, rsl.ErrRSLEntryNotFound) {
			slog.Debug(fmt.Sprintf("No policy found before initial entry '%s'", entry.GetID().String()))
			return nil, ErrPolicyNotFound
		}

		return nil, err
	}

	return policyEntry, nil
}

// FindFirstHooksEntry returns the first reference entry for HooksRef
// returns *rsl.ReferenceEntry
func (r *regularSearcher) FindFirstHooksEntry() (*rsl.ReferenceEntry, error) {
	entry, _, err := rsl.GetFirstReferenceEntryForRef(r.repo, HooksRef)
	if err != nil {
		if errors.Is(err, rsl.ErrRSLEntryNotFound) {
			// we don't have a policy entry yet
			return nil, ErrPolicyNotFound
		}
		return nil, err
	}

	return entry, nil
}

// LoadCurrentState returns the current state of teh repository as a
// HookState object.
func LoadCurrentState(repo *gitinterface.Repository) (*HookState, error) {
	entry, _, err := rsl.GetLatestReferenceEntry(repo, rsl.ForReference(HooksRef))
	if err != nil {
		return nil, err
	}
	fmt.Println(entry)
	return loadState(repo, entry)
}

// InitializeHooksMetadata initializes an empty hooks Metadata object
func InitializeHooksMetadata() Metadata {
	return Metadata{HooksInfo: make(map[string]*Hook), Bindings: make(map[string]string)}
}

// GetHooksMetadata returns the hooks Metadata associated with the current HookState
func (s *HookState) GetHooksMetadata() (*Metadata, error) {
	h := s.HooksEnvelope
	if h == nil {
		slog.Debug("Could not find requested metadata file; initializing hooks metadata")
		return nil, ErrMetadataNotFound
	}

	payloadBytes, err := h.DecodeB64Payload()
	if err != nil {
		return nil, err
	}

	slog.Debug(string(payloadBytes))
	hooksMetadata := &Metadata{}
	if err := json.Unmarshal(payloadBytes, hooksMetadata); err != nil {
		return nil, err
	}

	return hooksMetadata, nil
}

// GenerateMetadataFor writes Metadata about the provided file and returns an error, if any
func (h *Metadata) GenerateMetadataFor(hookName, stage, env string, blobID, sha256HashSum gitinterface.Hash, modules []string) error {
	hookInfo := Hook{
		SHA256Hash:  sha256HashSum.String(),
		Stage:       stage,
		BlobID:      blobID.String(),
		Environment: env,
		Modules:     modules,
	}
	h.HooksInfo[hookName] = &hookInfo
	h.Bindings[stage] = hookName
	return nil
}

// GetTargetsMetadata returns a tuf.TargetsMetadata object corresponding to the current HookState
func (s *HookState) GetTargetsMetadata(roleName string) (tuf.TargetsMetadata, error) {
	e := s.TargetsEnvelope
	if roleName != TargetsRoleName {
		env, ok := s.DelegationEnvelopes[roleName]
		if !ok {
			return nil, ErrMetadataNotFound
		}
		e = env
	}

	if e == nil {
		return nil, ErrMetadataNotFound
	}

	payloadBytes, err := e.DecodeB64Payload()
	if err != nil {
		return nil, err
	}

	targetsMetadata := &tufv01.TargetsMetadata{}
	if err := json.Unmarshal(payloadBytes, targetsMetadata); err != nil {
		return nil, err
	}

	return targetsMetadata, nil
}

// Commit writes hooks and targets metadata and file (if provided) and returns an error if any
func (s *HookState) Commit(repo *gitinterface.Repository, commitMessage, hookName string, addBlob gitinterface.Hash, sign bool) error {
	if len(commitMessage) == 0 {
		commitMessage = DefaultCommitMessage
	}

	metadata := map[string]*sslibdsse.Envelope{}
	if s.TargetsEnvelope != nil {
		metadata[TargetsRoleName] = s.TargetsEnvelope
	}
	if s.HooksEnvelope != nil {
		metadata[HooksRoleName] = s.HooksEnvelope
	}

	allTreeEntries := map[string]gitinterface.Hash{}
	for name, env := range metadata {
		envContents, err := json.Marshal(env)
		if err != nil {
			return err
		}

		blobID, err := repo.WriteBlob(envContents)
		if err != nil {
			return err
		}

		allTreeEntries[path.Join(metadataTreeEntryName, name+".json")] = blobID
	}

	if len(addBlob) > 0 {
		allTreeEntries[path.Join(hooksTreeEntryName, hookName)] = addBlob
	}

	slog.Debug("building and populating new tree...")
	treeBuilder := gitinterface.NewTreeBuilder(repo)

	hooksRootTreeID, err := treeBuilder.WriteRootTreeFromBlobIDs(allTreeEntries)
	if err != nil {
		return err
	}

	originalCommitID, err := repo.GetReference(HooksRef)
	if err != nil {
		if !errors.Is(err, gitinterface.ErrReferenceNotFound) {
			return err
		}
	}
	commitID, err := repo.Commit(hooksRootTreeID, HooksRef, commitMessage, sign)
	if err != nil {
		return err
	}
	slog.Debug("committing hooks metadata successful!")

	// record changes to RSL; reset to original policy commit if err != nil

	newReferenceEntry := rsl.NewReferenceEntry(HooksRef, commitID)
	if err := newReferenceEntry.Commit(repo, true); err != nil {
		if !originalCommitID.IsZero() {
			return repo.ResetDueToError(err, HooksRef, originalCommitID)
		}

		return err
	}
	slog.Debug("RSL entry recording successful!")
	hooksTip, err := repo.GetReference(HooksRef)
	if err := repo.SetReference(HooksRef, hooksTip); err != nil {
		return fmt.Errorf("failed to set new hooks reference: %w", err)
	}
	return nil
}

// HasBeenInitialized returns a boolean value signifying whether the hooks have been
// initialized or not. If hooks exist, return True.
func (s *HookState) HasBeenInitialized() bool {
	return s.HooksEnvelope != nil
}

func loadStateForEntry(repo *gitinterface.Repository, entry *rsl.ReferenceEntry) (*HookState, error) {
	if entry.RefName != HooksRef {
		return nil, rsl.ErrRSLEntryDoesNotMatchRef
	}

	commitTreeID, err := repo.GetCommitTreeID(entry.TargetID)
	if err != nil {
		return nil, err
	}

	allTreeEntries, err := repo.GetAllFilesInTree(commitTreeID)
	if err != nil {
		return nil, err
	}

	state := &HookState{Repository: repo}

	for name, blobID := range allTreeEntries {
		contents, err := repo.ReadBlob(blobID)
		if err != nil {
			return nil, err
		}

		if strings.HasPrefix(name, metadataTreeEntryName+"/") {
			env := &sslibdsse.Envelope{}
			if err := json.Unmarshal(contents, env); err != nil {
				return nil, err
			}

			metadataName := strings.TrimPrefix(name, metadataTreeEntryName+"/")
			switch metadataName {
			case fmt.Sprintf("%s.json", TargetsRoleName):
				state.TargetsEnvelope = env

			case fmt.Sprintf("%s.json", HooksRoleName):
				state.HooksEnvelope = env
			default:
				if state.DelegationEnvelopes == nil {
					state.DelegationEnvelopes = map[string]*sslibdsse.Envelope{}
				}

				state.DelegationEnvelopes[strings.TrimSuffix(metadataName, ".json")] = env
			}
		}
	}

	return state, nil
}

func loadState(repo *gitinterface.Repository, requestedEntry *rsl.ReferenceEntry) (*HookState, error) {
	searcher := newSearcher(repo)
	firstHooksEntry, err := searcher.FindFirstHooksEntry()
	if err != nil {
		if errors.Is(err, ErrPolicyNotFound) {
			return loadStateForEntry(repo, requestedEntry)
		}
		return nil, err
	}
	knows, err := repo.KnowsCommit(requestedEntry.ID, firstHooksEntry.ID) // this is the problem
	if err != nil {
		return nil, err
	}
	if knows {
		slog.Debug("knows")
		return loadStateForEntry(repo, requestedEntry)
	}

	initialHooksState, err := loadStateForEntry(repo, firstHooksEntry)
	if err != nil {
		return nil, err
	}

	return initialHooksState, nil
}
