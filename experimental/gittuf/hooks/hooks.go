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
	HooksRef              = "refs/gittuf/hooks"
	DefaultCommitMessage  = "Create hooks ref"
	RootRoleName          = "root"
	TargetsRoleName       = "targets"
	metadataTreeEntryName = "metadata"
	hooksTreeEntryName    = "hooks"
	HooksRoleName         = "hooks"
	ApplyMessage          = "Apply hooks"
)

var (
	ErrMetadataNotFound          = errors.New("unable to find requested metadata file")
	ErrPolicyNotFound            = errors.New("cannot find policy")
	ErrHooksMetadataHashMismatch = errors.New("error verifying Hooks metadata - hashes do not match")
)

// Hooks metadata should be encoded on existing policy metadata
// We can have a default policy that protects the hooks ref, and use this
// policy to encode the metadata

// TODO: check how to initialize a default policy and reserve the name.
// Use this policy to add hook path, key ids, etc.
// policy.AddDelegation or policy.InitializeTargetsMetadata
// check out policy/helpers_test.go, policy/policy_test.go and policy/targets_test.go
// for information about how to init

type StateWrapper struct {
	RootEnvelope        *sslibdsse.Envelope
	TargetsEnvelope     *sslibdsse.Envelope
	HooksEnvelope       *sslibdsse.Envelope
	DelegationEnvelopes map[string]*sslibdsse.Envelope
	RootPublicKeys      []tuf.Principal
	Repository          *gitinterface.Repository
}

type Metadata struct {
	HooksInfo map[string]*Information `json:"HooksInfo"`
	Bindings  map[string]string       `json:"Bindings"`
}

type Information struct {
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

		// Any other err must be returned
		return nil, err
	}

	return policyEntry, nil
}

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

func (r *regularSearcher) FindHooksEntriesInRange(firstEntry, lastEntry rsl.Entry) ([]*rsl.ReferenceEntry, error) {
	allPolicyEntries, _, err := rsl.GetReferenceEntriesInRangeForRef(r.repo, firstEntry.GetID(), lastEntry.GetID(), HooksRef)
	if err != nil {
		return nil, err
	}

	return allPolicyEntries, nil
}

func loadStateForEntry(repo *gitinterface.Repository, entry *rsl.ReferenceEntry) (*StateWrapper, error) {
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

	state := &StateWrapper{Repository: repo}

	for name, blobID := range allTreeEntries {
		contents, err := repo.ReadBlob(blobID)
		if err != nil {
			return nil, err
		}

		// We have this conditional because once upon a time we used to store
		// the root keys on disk as well; now we just get them from the root
		// metadata file. We ignore the keys on disk in the old policy states.
		if strings.HasPrefix(name, metadataTreeEntryName+"/") {
			env := &sslibdsse.Envelope{}
			if err := json.Unmarshal(contents, env); err != nil {
				return nil, err
			}

			metadataName := strings.TrimPrefix(name, metadataTreeEntryName+"/")
			switch metadataName {
			case fmt.Sprintf("%s.json", RootRoleName):
				state.RootEnvelope = env

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

func LoadState(repo *gitinterface.Repository, requestedEntry *rsl.ReferenceEntry) (*StateWrapper, error) {
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

func LoadCurrentState(repo *gitinterface.Repository) (*StateWrapper, error) {
	entry, _, err := rsl.GetLatestReferenceEntry(repo, rsl.ForReference(HooksRef))
	if err != nil {
		return nil, err
	}
	fmt.Println(entry)
	return LoadState(repo, entry)
}

// LoadFirstState returns the State corresponding to the first Hooks commit.
// Verification of RoT is skipped since it is the initial commit.
//func LoadFirstState(ctx context.Context, repo *gitinterface.Repository) (*StateWrapper, error) {
//	policyState, err := policy.LoadCurrentState(ctx, repo, HooksRef)
//	if err != nil {
//		return nil, err
//	}
//	slog.Debug("policyState fetch did not return err")
//	returnState := StateWrapper{
//		Repository:          repo,
//		TargetsEnvelope:     policyState.TargetsEnvelope,
//		DelegationEnvelopes: policyState.DelegationEnvelopes,
//	}
//	return &returnState, nil
//}

func InitializeHooksMetadata() Metadata {
	return Metadata{HooksInfo: make(map[string]*Information), Bindings: make(map[string]string)}
}

func (s *StateWrapper) GetHooksMetadata() (*Metadata, error) {
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

func (s *StateWrapper) Commit(repo *gitinterface.Repository, commitMessage, hookName string, addBlob gitinterface.Hash, sign bool) error {
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

func (h *Metadata) GenerateMetadataFor(hookName, stage, env string, blobID, sha256HashSum gitinterface.Hash, modules []string) error {
	hookInfo := Information{
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

func (s *StateWrapper) GetTargetsMetadata(roleName string) (tuf.TargetsMetadata, error) {
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

func (s *StateWrapper) HasBeenInitialized() bool {
	return s.HooksEnvelope != nil
}
