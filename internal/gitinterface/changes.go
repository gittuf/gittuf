package gitinterface

import (
	"errors"
	"fmt"
	"sort"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// GetCommitFilePaths returns all the file paths of the provided commit object.
func GetCommitFilePaths(repo *git.Repository, commit *object.Commit) ([]string, error) {
	filesIter, err := commit.Files()
	if err != nil {
		return nil, err
	}

	paths := []string{}
	if err := filesIter.ForEach(func(f *object.File) error {
		paths = append(paths, f.Name)
		return nil
	}); err != nil {
		return nil, err
	}

	sort.Slice(paths, func(i, j int) bool {
		return paths[i] < paths[j]
	})

	return paths, nil
}

func GetDiffFilePaths(repo *git.Repository, commitA, commitB *object.Commit) ([]string, error) {
	if commitA == nil && commitB == nil {
		return nil, fmt.Errorf("both commits cannot be empty")
	}

	if commitA == nil {
		return GetCommitFilePaths(repo, commitB)
	}
	if commitB == nil {
		return GetCommitFilePaths(repo, commitA)
	}

	treeAExists := true
	treeA, err := commitA.Tree()
	if err != nil {
		if errors.Is(err, plumbing.ErrObjectNotFound) {
			treeAExists = false
		} else {
			return nil, err
		}
	}

	treeBExists := true
	treeB, err := commitB.Tree()
	if err != nil {
		if errors.Is(err, plumbing.ErrObjectNotFound) {
			treeBExists = false
		} else {
			return nil, err
		}
	}

	if treeAExists && treeBExists {
		return diff(treeA, treeB)
	} else if treeAExists {
		return GetCommitFilePaths(repo, commitA)
	} else if treeBExists {
		return GetCommitFilePaths(repo, commitB)
	}

	// return empty and non-error if both have empty trees?
	return nil, nil
}

func diff(treeA, treeB *object.Tree) ([]string, error) {
	changesSet := map[string]bool{}
	changes, err := treeA.Diff(treeB)
	if err != nil {
		return nil, err
	}

	for _, c := range changes {
		if len(c.From.Name) > 0 {
			changesSet[c.From.Name] = true
		}
		if len(c.To.Name) > 0 {
			changesSet[c.To.Name] = true
		}
	}

	paths := []string{}
	for p := range changesSet {
		paths = append(paths, p)
	}

	sort.Slice(paths, func(i, j int) bool {
		return paths[i] < paths[j]
	})

	return paths, nil
}
