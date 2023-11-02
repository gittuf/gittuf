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
	Ref                         = "refs/gittuf/attestations"
	authorizationsTreeEntryName = "authorizations"
	initialCommitMessage        = "Initial commit"
	defaultCommitMessage        = "Update attestations"
)

var ErrAttestationsExist = errors.New("cannot initialize attestations namespace as it exists already")

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

type Attestations struct {
	authorizations map[string]plumbing.Hash // refPath/from-to -> blob ID
}

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
		// this happens in the initial commit for the attestations namespace,
		// with no entries in the tree at all
		return &Attestations{}, nil
	}

	var authorizationsTreeID plumbing.Hash
	for _, e := range attestationsRootTree.Entries {
		if e.Name == authorizationsTreeEntryName {
			authorizationsTreeID = e.Hash
		}
	}

	authorizationsTree, err := gitinterface.GetTree(repo, authorizationsTreeID)
	if err != nil {
		return nil, err
	}

	attestations := &Attestations{authorizations: map[string]plumbing.Hash{}}

	filesIter := authorizationsTree.Files()
	if err := filesIter.ForEach(func(f *object.File) error {
		attestations.authorizations[f.Name] = f.Blob.Hash
		return nil
	}); err != nil {
		return nil, err
	}

	return attestations, nil
}

func (a *Attestations) Commit(repo *git.Repository, commitMessage string, signCommit bool) error {
	if len(commitMessage) == 0 {
		commitMessage = defaultCommitMessage
	}

	attestationsTreeEntries := []object.TreeEntry{}
	treeBuilder := gitinterface.NewTreeBuilder(repo)

	// Add authorizations tree
	authorizationsTreeID, err := treeBuilder.WriteRootTreeFromBlobIDs(a.authorizations)
	if err != nil {
		return err
	}
	attestationsTreeEntries = append(attestationsTreeEntries, object.TreeEntry{
		Name: authorizationsTreeEntryName,
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
