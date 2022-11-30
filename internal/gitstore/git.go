package gitstore

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

const (
	Ref = "refs/gittuf/metadata"
)

/*
InitRepository is invoked during the init workflow. A set of TUF metadata is
created and passed in. This is then written to the store.
*/
func InitRepository(repoRoot string, metadata map[string][]byte) (*Repository, error) {
	err := InitNamespace(repoRoot)
	if err != nil {
		return &Repository{}, err
	}

	repo, err := git.PlainOpen(repoRoot)
	if err != nil {
		return &Repository{}, err
	}
	return &Repository{
		Metadata:            metadata,
		repository:          repo,
		tip:                 plumbing.ZeroHash,
		metadataIdentifiers: make(map[string]object.TreeEntry, len(metadata)),
		written:             false,
	}, nil
}

func LoadRepository(repoRoot string) (*Repository, error) {
	repo, err := git.PlainOpen(repoRoot)
	if err != nil {
		return &Repository{}, err
	}
	ref, err := repo.Reference(plumbing.ReferenceName(Ref), true)
	if err != nil {
		return &Repository{}, err
	}

	if ref.Hash() == plumbing.ZeroHash {
		return &Repository{
			Metadata:            map[string][]byte{},
			repository:          repo,
			tip:                 plumbing.ZeroHash,
			tree:                plumbing.ZeroHash,
			metadataIdentifiers: map[string]object.TreeEntry{},
			written:             true,
		}, nil
	}

	commitObj, err := repo.CommitObject(ref.Hash())
	if err != nil {
		return &Repository{}, err
	}

	tree, err := repo.TreeObject(commitObj.TreeHash)
	if err != nil {
		return &Repository{}, err
	}

	metadata := map[string][]byte{}
	metadataIdentifiers := map[string]object.TreeEntry{}
	for _, entry := range tree.Entries {
		// FIXME: Assuming everything is a blob
		_, contents, err := readBlob(repo, entry.Hash)
		if err != nil {
			return &Repository{}, err
		}

		metadataIdentifiers[entry.Name] = entry
		metadata[entry.Name] = contents
	}

	return &Repository{
		Metadata:            metadata,
		repository:          repo,
		tip:                 commitObj.Hash,
		tree:                commitObj.TreeHash,
		metadataIdentifiers: metadataIdentifiers,
		written:             true,
	}, nil
}

type Repository struct {
	// FIXME: We likely don't need the public Metadata field, it's added in-memory complexity
	Metadata            map[string][]byte // filename: contents
	repository          *git.Repository
	tip                 plumbing.Hash
	tree                plumbing.Hash
	metadataIdentifiers map[string]object.TreeEntry // filename: TreeEntry object
	written             bool
}

func (r *Repository) Tip() string {
	return r.tip.String()
}

func (r *Repository) Tree() (*object.Tree, error) {
	return r.repository.TreeObject(r.tree)
}

func (r *Repository) Written() bool {
	return r.written
}

func (r *Repository) GetCurrentFileBytes(name string) []byte {
	return r.Metadata[name]
}

func (r *Repository) GetCurrentFileString(name string) string {
	return string(r.Metadata[name])
}

func (r *Repository) Stage(name string, contents []byte) {
	r.Metadata[name] = contents
	r.written = false
}

func (r *Repository) StageAndCommit(name string, contents []byte) error {
	r.Stage(name, contents)
	return r.CommitHeldMetadata()
}

func (r *Repository) StageMultiple(metadata map[string][]byte) {
	for name, contents := range metadata {
		r.Stage(name, contents)
	}
}

func (r *Repository) StageAndCommitMultiple(metadata map[string][]byte) error {
	r.StageMultiple(metadata)
	return r.CommitHeldMetadata()
}

func (r *Repository) CommitHeldMetadata() error {
	currentEntries := make([]object.TreeEntry, 0, len(r.Metadata))

	// Write held blobs
	for metadata, contents := range r.Metadata {
		identifier, err := writeBlob(r.repository, contents)
		if err != nil {
			return err
		}
		entry := object.TreeEntry{
			Name: metadata,
			Mode: 0644,
			Hash: identifier,
		}
		r.metadataIdentifiers[metadata] = entry
		currentEntries = append(currentEntries, entry)
	}

	// Create a new tree object
	treeHash, err := writeTree(r.repository, currentEntries)
	if err != nil {
		return err
	}
	r.tree = treeHash

	// Commit to ref
	commitHash, err := commit(r.repository, r.tip, treeHash, Ref)
	if err != nil {
		return err
	}
	r.tip = commitHash

	r.written = true

	return nil
}

func InitNamespace(repoRoot string) error {
	_, err := os.Stat(filepath.Join(repoRoot, ".git", Ref))
	if os.IsNotExist(err) {
		err := os.Mkdir(filepath.Join(repoRoot, ".git", "refs", "gittuf"), 0755)
		if err != nil {
			return err
		}
		err = os.WriteFile(filepath.Join(repoRoot, ".git", Ref), plumbing.ZeroHash[:], 0644)
		if err != nil {
			return err
		}
	}
	return nil
}

func writeBlob(repo *git.Repository, contents []byte) (plumbing.Hash, error) {
	obj := repo.Storer.NewEncodedObject()
	obj.SetType(plumbing.BlobObject)
	writer, err := obj.Writer()
	if err != nil {
		return plumbing.ZeroHash, err
	}
	_, err = writer.Write(contents)
	if err != nil {
		return plumbing.ZeroHash, err
	}
	return repo.Storer.SetEncodedObject(obj)
}

func writeTree(repo *git.Repository, entries []object.TreeEntry) (plumbing.Hash, error) {
	obj := repo.Storer.NewEncodedObject()
	tree := object.Tree{
		Entries: entries,
	}
	err := tree.Encode(obj)
	if err != nil {
		return plumbing.ZeroHash, err
	}
	return repo.Storer.SetEncodedObject(obj)
}

func commit(repo *git.Repository, parent plumbing.Hash, treeHash plumbing.Hash, targetRef string) (plumbing.Hash, error) {
	gitConfig, err := repo.ConfigScoped(config.GlobalScope)
	if err != nil {
		return plumbing.ZeroHash, err
	}

	curRef, err := repo.Reference(plumbing.ReferenceName(targetRef), true)
	if err != nil {
		return plumbing.ZeroHash, err
	}

	author := object.Signature{
		Name:  gitConfig.User.Name,
		Email: gitConfig.User.Email,
		When:  time.Now(),
	}

	commit := object.Commit{
		Author:    author,
		Committer: author,
		TreeHash:  treeHash,
		Message:   fmt.Sprintf("gittuf: Writing metadata tree %s", treeHash.String()),
	}
	if parent != plumbing.ZeroHash {
		commit.ParentHashes = []plumbing.Hash{parent}
	}

	obj := repo.Storer.NewEncodedObject()
	err = commit.Encode(obj)
	if err != nil {
		return plumbing.ZeroHash, err
	}
	commitHash, err := repo.Storer.SetEncodedObject(obj)
	if err != nil {
		return plumbing.ZeroHash, err
	}

	newRef := plumbing.NewHashReference(plumbing.ReferenceName(targetRef), commitHash)
	err = repo.Storer.CheckAndSetReference(newRef, curRef)
	return commitHash, err
}

func readBlob(repo *git.Repository, blobHash plumbing.Hash) (int, []byte, error) {
	blob, err := repo.BlobObject(blobHash)
	if err != nil {
		return -1, []byte{}, err
	}
	contents := make([]byte, blob.Size)
	reader, err := blob.Reader()
	if err != nil {
		return -1, []byte{}, err
	}
	length, err := reader.Read(contents)
	if err != nil {
		return -1, []byte{}, err
	}
	return length, contents, nil
}
