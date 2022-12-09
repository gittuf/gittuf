package gitstore

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	tufdata "github.com/theupdateframework/go-tuf/data"
)

const LastTrustedRef = "refs/gittuf/last-trusted"

func InitNamespace(repoRoot string) error {
	// FIXME: this does not handle detached gitdir?
	_, err := os.Stat(filepath.Join(repoRoot, ".git", StateRef))
	if os.IsNotExist(err) {
		err := os.Mkdir(filepath.Join(repoRoot, ".git", "refs", "gittuf"), 0755)
		if err != nil {
			return err
		}
		err = os.WriteFile(filepath.Join(repoRoot, ".git", StateRef), plumbing.ZeroHash[:], 0644)
		if err != nil {
			return err
		}
		err = os.WriteFile(filepath.Join(repoRoot, ".git", LastTrustedRef), plumbing.ZeroHash[:], 0644)
		if err != nil {
			return err
		}
	}
	return nil
}

type GitStore struct {
	repository  *git.Repository
	state       *State
	lastTrusted plumbing.Hash
}

func InitGitStore(repoRoot string, rootPublicKeys []tufdata.PublicKey, metadata map[string][]byte) (*GitStore, error) {
	err := InitNamespace(repoRoot)
	if err != nil {
		return &GitStore{}, err
	}

	repo, err := git.PlainOpen(repoRoot)
	if err != nil {
		return &GitStore{}, err
	}

	state, err := initState(repo, rootPublicKeys, metadata)
	if err != nil {
		return &GitStore{}, err
	}

	return &GitStore{
		repository:  repo,
		state:       state,
		lastTrusted: plumbing.ZeroHash,
	}, nil
}

func LoadGitStore(repoRoot string) (*GitStore, error) {
	repo, err := git.PlainOpen(repoRoot)
	if err != nil {
		return &GitStore{}, err
	}

	stateRef, err := repo.Reference(plumbing.ReferenceName(StateRef), true)
	if err != nil {
		return &GitStore{}, err
	}

	if stateRef.Hash().IsZero() {
		return &GitStore{
			repository: repo,
			state: &State{
				metadataStaging:     map[string][]byte{},
				keysStaging:         map[string][]byte{},
				repository:          repo,
				tip:                 plumbing.ZeroHash,
				tree:                plumbing.ZeroHash,
				metadataIdentifiers: map[string]object.TreeEntry{},
				rootKeys:            map[string]object.TreeEntry{},
				written:             true,
			},
			lastTrusted: plumbing.ZeroHash,
		}, nil
	}

	state, err := loadState(repo, stateRef.Hash())
	if err != nil {
		return &GitStore{}, err
	}

	lastTrustedRef, err := repo.Reference(plumbing.ReferenceName(LastTrustedRef), true)
	if err != nil {
		return &GitStore{}, err
	}

	return &GitStore{
		repository:  repo,
		state:       state,
		lastTrusted: lastTrustedRef.Hash(),
	}, nil
}

func (g *GitStore) GetLastTrusted() (map[string]string, error) {
	_, contents, err := readBlob(g.repository, g.lastTrusted)
	if err != nil {
		return map[string]string{}, err
	}
	var lastTrusted map[string]string
	err = json.Unmarshal(contents, &lastTrusted)
	return lastTrusted, err
}

func (g *GitStore) WriteLastTrusted(lastTrusted map[string]string) error {
	contents, err := json.Marshal(lastTrusted)
	if err != nil {
		return err
	}
	contentID, err := writeBlob(g.repository, contents)
	if err != nil {
		return err
	}

	g.lastTrusted = contentID
	oldRef, err := g.repository.Reference(plumbing.ReferenceName(LastTrustedRef), true)
	if err != nil {
		return err
	}
	newRef := plumbing.NewHashReference(plumbing.ReferenceName(LastTrustedRef), contentID)

	return g.repository.Storer.CheckAndSetReference(newRef, oldRef)
}

func (g *GitStore) State() *State {
	return g.state
}
