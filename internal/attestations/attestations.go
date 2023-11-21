// SPDX-License-Identifier: Apache-2.0

package attestations

import (
	"errors"

	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/gittuf/gittuf/internal/third_party/go-git"
	"github.com/gittuf/gittuf/internal/third_party/go-git/plumbing"
	"github.com/gittuf/gittuf/internal/third_party/go-git/plumbing/filemode"
	"github.com/gittuf/gittuf/internal/third_party/go-git/plumbing/object"
)

const (
	Ref                                  = "refs/gittuf/attestations"
	referenceAuthorizationsTreeEntryName = "reference-authorizations"
	initialCommitMessage                 = "Initial commit"
	defaultCommitMessage                 = "Update attestations"
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
	// `refs/heads/main/<commit-A>-<commit-B>` indicates the authorization is
	// for the action of moving `refs/heads/main` from `commit-A` to `commit-B`.
	referenceAuthorizations map[string]plumbing.Hash
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
		// with no entries in the tree at all
		// An RSL entry isn't created by default for that entry but if one is,
		// we don't want to panic.
		return &Attestations{}, nil
	}

	var authorizationsTreeID plumbing.Hash
	for _, e := range attestationsRootTree.Entries {
		if e.Name == referenceAuthorizationsTreeEntryName {
			authorizationsTreeID = e.Hash
		}
	}

	authorizationsTree, err := gitinterface.GetTree(repo, authorizationsTreeID)
	if err != nil {
		return nil, err
	}

	attestations := &Attestations{referenceAuthorizations: map[string]plumbing.Hash{}}

	attestations.referenceAuthorizations, err = gitinterface.GetAllFilesInTree(authorizationsTree)
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
