package repository

import (
	"github.com/adityasaky/gittuf/internal/common"
	"github.com/adityasaky/gittuf/internal/policy"
	"github.com/adityasaky/gittuf/internal/rsl"
	"github.com/go-git/go-git/v5"
)

type Repository struct {
	r *git.Repository
}

func LoadRepository() (*Repository, error) {
	repo, err := common.GetRepositoryHandler()
	if err != nil {
		return nil, err
	}

	return &Repository{
		r: repo,
	}, nil
}

func (r *Repository) InitializeNamespaces() error {
	if err := rsl.InitializeNamespace(r.r); err != nil {
		return err
	}

	return policy.InitializeNamespace(r.r)
}
