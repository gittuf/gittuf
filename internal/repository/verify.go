package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/adityasaky/gittuf/internal/policy"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

var (
	ErrReferenceNotFound = errors.New("reference not found")
)

func (r *Repository) Verify(ctx context.Context, target string) error {
	target, err := absoluteReference(r.r, target)
	if err != nil {
		return err
	}

	return policy.VerifyRef(ctx, r.r, target)
}

func absoluteReference(repo *git.Repository, target string) (string, error) {
	if strings.HasPrefix(target, "refs/") {
		return target, nil
	}

	// Check if branch
	refName := plumbing.NewBranchReferenceName(target)
	_, err := repo.Reference(refName, false)
	if err != nil {
		if !errors.Is(err, plumbing.ErrReferenceNotFound) {
			return "", err
		}
	} else {
		return string(refName), nil
	}

	// Check if tag
	refName = plumbing.NewTagReferenceName(target)
	_, err = repo.Reference(refName, false)
	if err != nil {
		if !errors.Is(err, plumbing.ErrReferenceNotFound) {
			return "", err
		}
	} else {
		return string(refName), nil
	}

	return "", ErrReferenceNotFound
}
