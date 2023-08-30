package repository

import (
	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/go-git/go-git/v5/config"
)

func (r *Repository) Push(remoteName string, refNames ...string) error {
	refs := make([]config.RefSpec, 0, len(refNames)+1) // The +1 is for the RSL ref

	for _, refName := range refNames {
		absRefName, err := absoluteReference(r.r, refName)
		if err != nil {
			return err
		}
		refs = append(refs, refSpec(absRefName, false))
	}

	refs = append(refs, refSpec(rsl.RSLRef, false)) // we always want to sync the RSL ref

	return gitinterface.Push(r.r, remoteName, refs)
}
