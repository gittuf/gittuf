package repository

import (
	"context"
	"errors"
	"net/url"
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
	ErrPullingRepository = errors.New("unable to pull from remote")
)

// Clone wraps a typical git clone invocation, fetching gittuf refs in addition
// to the standard refs. It performs a verification of the RSL against the
// specified HEAD after cloning the repository.
// TODO: resolve how root keys are trusted / bootstrapped.
func Clone(ctx context.Context, remoteURL, dir, initialBranch string) (*Repository, error) {
	if dir == "" {
		dir = getLocalDirName(remoteURL)
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

	headRef, err := gitinterface.AbsoluteReference(r, "HEAD")
	if err != nil {
		return nil, errors.Join(ErrCloningRepository, err)
	}

	repository := &Repository{r: r}
	return repository, repository.VerifyRef(ctx, headRef, true)
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

// Pull wraps a typical git pull invocation by also fetching gittuf namespaces
// from the remote.
func (r *Repository) Pull(ctx context.Context, remoteName string, refNames ...string) error {
	updatedRefNames := append(refNames, rsl.RSLRef, policy.PolicyRef)
	if err := gitinterface.Pull(ctx, r.r, remoteName, updatedRefNames); err != nil {
		return errors.Join(ErrPullingRepository, err)
	}

	for _, ref := range refNames {
		if err := r.VerifyRef(ctx, ref, true); err != nil {
			return errors.Join(ErrPullingRepository, err)
		}
	}

	return nil
}

func getLocalDirName(remoteURL string) string {
	var path string
	rURL, err := url.Parse(remoteURL)
	if err != nil {
		// We suppress errors and try to find the dir using the URL itself
		// git@github.com:gittuf/gittuf triggers an error for example due to the
		// colon
		path = remoteURL
	} else {
		path = rURL.EscapedPath()
		if len(path) == 0 {
			// https://go.dev/play/p/eEoLuwxWE2F
			// In Windows paths, the escapedPath is empty
			path = remoteURL
		}
	}

	split := strings.Split(strings.TrimSpace(strings.ReplaceAll(path, "\\", "/")), "/")
	return strings.TrimSuffix(split[len(split)-1], ".git")
}
