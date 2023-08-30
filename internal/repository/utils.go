package repository

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
)

var ErrReferenceNotFound = errors.New("reference not found")

func absoluteReference(repo *git.Repository, target string) (string, error) {
	if strings.HasPrefix(target, "refs/") {
		return target, nil
	}

	// Check if branch
	refName := plumbing.NewBranchReferenceName(target)
	_, err := repo.Reference(refName, false)
	if err == nil {
		return string(refName), nil
	}
	if !errors.Is(err, plumbing.ErrReferenceNotFound) {
		return "", err
	}

	// Check if tag
	refName = plumbing.NewTagReferenceName(target)
	_, err = repo.Reference(refName, false)
	if err == nil {
		return string(refName), nil
	}
	if !errors.Is(err, plumbing.ErrReferenceNotFound) {
		return "", err
	}

	return "", ErrReferenceNotFound
}

func refSpec(refName string, fastForwardOnly bool) config.RefSpec {
	var refString string

	found := strings.Contains(refName, ":")
	if found {
		refString = refName
	} else {
		refString = fmt.Sprintf("%s:%s", refName, refName)
	}

	if fastForwardOnly {
		return config.RefSpec(refString)
	}

	return config.RefSpec(fmt.Sprintf("+%s", refString))
}
