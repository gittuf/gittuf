// SPDX-License-Identifier: Apache-2.0

package attestations

import (
	"errors"
	"path"
	"strings"

	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/rsl"
)

const (
	Ref                                        = "refs/gittuf/attestations"
	referenceAuthorizationsTreeEntryName       = "reference-authorizations"
	githubPullRequestAttestationsTreeEntryName = "github-pull-requests"
	initialCommitMessage                       = "Initial commit"
	defaultCommitMessage                       = "Update attestations"
)

// Attestations tracks all the attestations in a gittuf repository.
type Attestations struct {
	// referenceAuthorizations maps each authorized action to the blob ID of the
	// attestation. The key is a path of the form
	// `<ref-path>/<from-id>-<to-id>`, where `ref-path` is the absolute ref path
	// such as `refs/heads/main` and `from-id` and `to-id` determine how the ref
	// in question moved. For example, the key
	// `refs/heads/main/<commit-A>-<tree-B>` indicates the authorization is
	// for the action of moving `refs/heads/main` from `commit-A` to a commit
	// with `tree-B`.
	referenceAuthorizations map[string]gitinterface.Hash

	// githubPullRequestAttestations maps information about the GitHub pull
	// request for a commit and branch. The key is a path of the form
	// `<ref-path>/<commit-id>`, where `ref-path` is the absolute ref path, and
	// `commit-id` is the ID of the merged commit.
	githubPullRequestAttestations map[string]gitinterface.Hash
}

// LoadCurrentAttestations inspects the repository's attestations namespace and
// loads the current attestations.
func LoadCurrentAttestations(repo *gitinterface.Repository) (*Attestations, error) {
	entry, _, err := rsl.GetLatestReferenceEntryForRef(repo, Ref)
	if err != nil {
		if !errors.Is(err, rsl.ErrRSLEntryNotFound) {
			return nil, err
		}

		return &Attestations{}, nil
	}

	return LoadAttestationsForEntry(repo, entry)
}

// LoadAttestationsForEntry loads the repository's attestations for a particular
// RSL entry for the attestations namespace.
func LoadAttestationsForEntry(repo *gitinterface.Repository, entry *rsl.ReferenceEntry) (*Attestations, error) {
	if entry.RefName != Ref {
		return nil, rsl.ErrRSLEntryDoesNotMatchRef
	}

	attestationsRootTreeID, err := repo.GetCommitTreeID(entry.TargetID)
	if err != nil {
		return nil, err
	}

	treeContents, err := repo.GetAllFilesInTree(attestationsRootTreeID)
	if err != nil {
		return nil, err
	}

	if len(treeContents) == 0 {
		// This happens in the initial commit for the attestations namespace,
		// where there are no entries in the tree yet.
		// This is expected, and there is nothing more to check so return a zero Attestations state.
		return &Attestations{}, nil
	}

	attestations := &Attestations{
		referenceAuthorizations:       map[string]gitinterface.Hash{},
		githubPullRequestAttestations: map[string]gitinterface.Hash{},
	}

	for name, blobID := range treeContents {
		switch {
		case strings.HasPrefix(name, referenceAuthorizationsTreeEntryName+"/"):
			attestations.referenceAuthorizations[strings.TrimPrefix(name, referenceAuthorizationsTreeEntryName+"/")] = blobID
		case strings.HasPrefix(name, githubPullRequestAttestationsTreeEntryName+"/"):
			attestations.githubPullRequestAttestations[strings.TrimPrefix(name, githubPullRequestAttestationsTreeEntryName+"/")] = blobID
		}
	}

	return attestations, nil
}

// LoadAttestationsForEntry loads the repository's attestations for a particular
// RSL entry for the attestations namespace.
func LoadAttestationsForEntryDirect(repo *gitinterface.Repository, entry gitinterface.Hash) (*Attestations, error) {
	attestationsRootTreeID, err := repo.GetCommitTreeID(entry)
	if err != nil {
		return nil, err
	}

	treeContents, err := repo.GetAllFilesInTree(attestationsRootTreeID)
	if err != nil {
		return nil, err
	}

	if len(treeContents) == 0 {
		// This happens in the initial commit for the attestations namespace,
		// where there are no entries in the tree yet.
		// This is expected, and there is nothing more to check so return a zero Attestations state.
		return &Attestations{}, nil
	}

	attestations := &Attestations{
		referenceAuthorizations:       map[string]gitinterface.Hash{},
		githubPullRequestAttestations: map[string]gitinterface.Hash{},
	}

	for name, blobID := range treeContents {
		switch {
		case strings.HasPrefix(name, referenceAuthorizationsTreeEntryName+"/"):
			attestations.referenceAuthorizations[strings.TrimPrefix(name, referenceAuthorizationsTreeEntryName+"/")] = blobID
		case strings.HasPrefix(name, githubPullRequestAttestationsTreeEntryName+"/"):
			attestations.githubPullRequestAttestations[strings.TrimPrefix(name, githubPullRequestAttestationsTreeEntryName+"/")] = blobID
		}
	}

	return attestations, nil
}

// Commit writes the state of the attestations to the repository, creating a new
// commit with the changes made. An RSL entry is also recorded for the
// namespace.
func (a *Attestations) Commit(repo *gitinterface.Repository, commitMessage string, signCommit bool) error {
	if len(commitMessage) == 0 {
		commitMessage = defaultCommitMessage
	}

	treeBuilder := gitinterface.NewTreeBuilder(repo)

	allAttestations := map[string]gitinterface.Hash{}
	for name, blobID := range a.referenceAuthorizations {
		allAttestations[path.Join(referenceAuthorizationsTreeEntryName, name)] = blobID
	}
	for name, blobID := range a.githubPullRequestAttestations {
		allAttestations[path.Join(githubPullRequestAttestationsTreeEntryName, name)] = blobID
	}

	attestationsTreeID, err := treeBuilder.WriteRootTreeFromBlobIDs(allAttestations)
	if err != nil {
		return err
	}

	priorCommitID, err := repo.GetReference(Ref)
	if err != nil {
		if !errors.Is(err, gitinterface.ErrReferenceNotFound) {
			return err
		}
	}

	newCommitID, err := repo.Commit(attestationsTreeID, Ref, commitMessage, signCommit)
	if err != nil {
		return err
	}

	// We must reset to original attestation commit if err != nil from here onwards.

	if err := rsl.NewReferenceEntry(Ref, newCommitID).Commit(repo, signCommit); err != nil {
		if !priorCommitID.IsZero() {
			return repo.ResetDueToError(err, Ref, priorCommitID)
		}

		return err
	}

	return nil
}
