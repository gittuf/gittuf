package repository

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/rsl"
)

var (
	ErrCloningRepository = errors.New("unable to clone repository")
	ErrDirExists         = errors.New("directory exists")
	ErrPushingRepository = errors.New("unable to push repository")
)

// Clone wraps a typical git clone invocation, fetching gittuf refs in addition
// to the standard refs. It performs a verification of the RSL against the
// specified HEAD after cloning the repository.
// TODO: resolve how root keys are trusted / bootstrapped.
func Clone(ctx context.Context, remoteURL, dir, initialBranch string) (*Repository, error) {
	if dir == "" {
		// FIXME: my understanding is backslashes are not used in URLs but I haven't dived into the RFCs to check yet
		split := strings.Split(strings.TrimSpace(strings.ReplaceAll(remoteURL, "\\", "/")), "/")
		dir = strings.TrimSuffix(split[len(split)-1], ".git")
	}
	_, err := os.Stat(dir)
	if err == nil {
		return nil, errors.Join(ErrCloningRepository, ErrDirExists)
	} else {
		if !os.IsNotExist(err) {
			return nil, errors.Join(ErrCloningRepository, err)
		}
	}

	if err := os.Mkdir(dir, 0755); err != nil {
		return nil, errors.Join(ErrCloningRepository, err)
	}

	refs := []string{rsl.RSLRef, policy.PolicyRef}

	r, err := gitinterface.CloneAndFetch(ctx, remoteURL, dir, initialBranch, refs, false)
	if err != nil {
		if e := os.RemoveAll(dir); e != nil {
			return nil, errors.Join(ErrCloningRepository, err, e)
		}
		return nil, errors.Join(ErrCloningRepository, err)
	}
	head, err := r.Reference(plumbing.HEAD, false)
	if err != nil {
		return nil, errors.Join(ErrCloningRepository, err)
	}

	repository := &Repository{r: r}
	return repository, repository.VerifyRef(ctx, head.Name().String(), true)
}

// Push wraps a typical git push invocation by also pushing gittuf namespaces to
// the remote.
func (r *Repository) Push(ctx context.Context, remoteName string, refNames ...string) error {
	for _, ref := range refNames {
		if err := r.VerifyRef(ctx, ref, true); err != nil {
			return errors.Join(ErrPushingRepository, err)
		}
	}
	refNames = append(refNames, rsl.RSLRef, policy.PolicyRef)
	err := gitinterface.Push(ctx, r.r, remoteName, refNames)
	if err != nil {
		return errors.Join(ErrPushingRepository, err)
	}

	return nil
}
