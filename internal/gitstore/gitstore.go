package gitstore

import (
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5/plumbing"
)

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
