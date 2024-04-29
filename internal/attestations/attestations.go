// SPDX-License-Identifier: Apache-2.0

package attestations

import (
	"errors"

	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
)

const (
	Ref                                        = "refs/gittuf/attestations"
	referenceAuthorizationsTreeEntryName       = "reference-authorizations"
	githubPullRequestAttestationsTreeEntryName = "github-pull-requests"
	initialCommitMessage                       = "Initial commit"
	defaultCommitMessage                       = "Update attestations"
)

var ErrAttestationsExist = errors.New("cannot initialize attestations namespace as it exists already")

// InitializeNamespace creates a namespace to store attestations for
// verification with gittuf. The ref is created with an initial, unsigned commit
// that is unsigned.
func InitializeNamespace(repo *git.Repository) error {
	if ref, err := repo.Reference(plumbing.ReferenceName(Ref), true); err != nil {
		if !errors.Is(err, plumbing.ErrReferenceNotFound) {
			return err
		}
	} else if !ref.Hash().IsZero() {
		return ErrAttestationsExist
	}

	treeHash, err := gitinterface.WriteTree(repo, nil)
	if err != nil {
		return err
	}

	_, err = gitinterface.Commit(repo, treeHash, Ref, initialCommitMessage, false)
	return err
}

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
	referenceAuthorizations map[string]plumbing.Hash

	// githubPullRequestAttestations maps information about the GitHub pull
	// request for a commit and branch. The key is a path of the form
	// `<ref-path>/<commit-id>`, where `ref-path` is the absolute ref path, and
	// `commit-id` is the ID of the merged commit.
	githubPullRequestAttestations map[string]plumbing.Hash
}

// LoadCurrentAttestations inspects the repository's attestations namespace and
// loads the current attestations.
func LoadCurrentAttestations(repo *git.Repository) (*Attestations, error) {
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
func LoadAttestationsForEntry(repo *git.Repository, entry *rsl.ReferenceEntry) (*Attestations, error) {
	if entry.RefName != Ref {
		return nil, rsl.ErrRSLEntryDoesNotMatchRef
	}

	attestationsCommit, err := gitinterface.GetCommit(repo, entry.TargetID)
	if err != nil {
		return nil, err
	}

	attestationsRootTree, err := gitinterface.GetTree(repo, attestationsCommit.TreeHash)
	if err != nil {
		return nil, err
	}

	if len(attestationsRootTree.Entries) == 0 {
		// This happens in the initial commit for the attestations namespace,
		// where there are no entries in the tree yet.
		// This is expected, and there is nothing more to check so return a zero Attestations state.
		return &Attestations{}, nil
	}

	var (
		authorizationsTreeID     plumbing.Hash
		githubPullRequestsTreeID plumbing.Hash
	)

	for _, e := range attestationsRootTree.Entries {
		if e.Name == referenceAuthorizationsTreeEntryName {
			authorizationsTreeID = e.Hash
		} else if e.Name == githubPullRequestAttestationsTreeEntryName {
			githubPullRequestsTreeID = e.Hash
		}
	}

	authorizationsTree, err := gitinterface.GetTree(repo, authorizationsTreeID)
	if err != nil {
		return nil, err
	}

	githubPullRequestsTree, err := gitinterface.GetTree(repo, githubPullRequestsTreeID)
	if err != nil {
		return nil, err
	}

	attestations := &Attestations{
		referenceAuthorizations:       map[string]plumbing.Hash{},
		githubPullRequestAttestations: map[string]plumbing.Hash{},
	}

	attestations.referenceAuthorizations, err = gitinterface.GetAllFilesInTree(authorizationsTree)
	if err != nil {
		return nil, err
	}

	attestations.githubPullRequestAttestations, err = gitinterface.GetAllFilesInTree(githubPullRequestsTree)
	if err != nil {
		return nil, err
	}

	return attestations, nil
}

// Commit writes the state of the attestations to the repository, creating a new
// commit with the changes made. An RSL entry is also recorded for the
// namespace.
func (a *Attestations) Commit(repo *git.Repository, commitMessage string, signCommit bool) error {
	if len(commitMessage) == 0 {
		commitMessage = defaultCommitMessage
	}

	attestationsTreeEntries := []object.TreeEntry{}
	treeBuilder := gitinterface.NewTreeBuilder(repo)

	// Add authorizations tree
	authorizationsTreeID, err := treeBuilder.WriteRootTreeFromBlobIDs(a.referenceAuthorizations)
	if err != nil {
		return err
	}
	attestationsTreeEntries = append(attestationsTreeEntries, object.TreeEntry{
		Name: referenceAuthorizationsTreeEntryName,
		Mode: filemode.Dir,
		Hash: authorizationsTreeID,
	})

	// Add GitHub pull requests tree
	githubPullRequestsTreeID, err := treeBuilder.WriteRootTreeFromBlobIDs(a.githubPullRequestAttestations)
	if err != nil {
		return err
	}
	attestationsTreeEntries = append(attestationsTreeEntries, object.TreeEntry{
		Name: githubPullRequestAttestationsTreeEntryName,
		Mode: filemode.Dir,
		Hash: githubPullRequestsTreeID,
	})

	attestationsTreeID, err := gitinterface.WriteTree(repo, attestationsTreeEntries)
	if err != nil {
		return err
	}

	ref, err := repo.Reference(plumbing.ReferenceName(Ref), true)
	if err != nil {
		return err
	}
	priorCommitID := ref.Hash()

	commitID, err := gitinterface.Commit(repo, attestationsTreeID, Ref, commitMessage, signCommit)
	if err != nil {
		return err
	}

	// We must reset to original attestation commit if err != nil from here onwards.

	if err := rsl.NewReferenceEntry(Ref, commitID).Commit(repo, signCommit); err != nil {
		return gitinterface.ResetDueToError(err, repo, Ref, priorCommitID)
	}

	return nil
}
