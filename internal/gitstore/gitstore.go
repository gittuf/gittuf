package gitstore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
	tufdata "github.com/theupdateframework/go-tuf/data"
)

const (
	Ref         = "refs/gittuf/state"
	MetadataDir = "metadata"
	KeysDir     = "keys"
)

/*
InitRepository is invoked during the init workflow. A set of TUF metadata is
created and passed in. This is then written to the store.
*/
func InitRepository(repoRoot string, rootPublicKeys []tufdata.PublicKey, metadata map[string][]byte) (*Repository, error) {
	err := InitNamespace(repoRoot)
	if err != nil {
		return &Repository{}, err
	}

	repo, err := git.PlainOpen(repoRoot)
	if err != nil {
		return &Repository{}, err
	}

	r := &Repository{
		metadataStaging:     map[string][]byte{},
		keysStaging:         map[string][]byte{},
		tip:                 plumbing.ZeroHash,
		tree:                plumbing.ZeroHash,
		repository:          repo,
		metadataIdentifiers: map[string]object.TreeEntry{},
		rootKeys:            map[string]object.TreeEntry{},
		written:             false,
	}

	err = r.StageKeys(rootPublicKeys)
	if err != nil {
		return &Repository{}, err
	}

	r.StageMultipleMetadata(metadata)

	return r, nil
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
			metadataStaging:     map[string][]byte{},
			keysStaging:         map[string][]byte{},
			repository:          repo,
			tip:                 plumbing.ZeroHash,
			tree:                plumbing.ZeroHash,
			metadataIdentifiers: map[string]object.TreeEntry{},
			rootKeys:            map[string]object.TreeEntry{},
			written:             true,
		}, nil
	}

	return loadRepository(repo, ref.Hash())
}

func LoadRepositoryAtState(repoRoot string, stateID string) (*Repository, error) {
	repo, err := git.PlainOpen(repoRoot)
	if err != nil {
		return &Repository{}, nil
	}
	ref, err := repo.Reference(plumbing.ReferenceName(Ref), true)
	if err != nil {
		return &Repository{}, err
	}

	currentHash := ref.Hash()
	stateHash := plumbing.NewHash(stateID)

	if stateHash == plumbing.ZeroHash || currentHash == plumbing.ZeroHash {
		return &Repository{}, fmt.Errorf("can't load gittuf repository at state zero")
	}
	if currentHash == stateHash {
		return LoadRepository(repoRoot)
	}

	// Check if stateHash is present when tracing back from currentHash
	iteratorHash := currentHash
	for {
		if iteratorHash == stateHash {
			break
		}

		commitObj, err := repo.CommitObject(iteratorHash)
		if err != nil {
			return &Repository{}, err
		}

		if len(commitObj.ParentHashes) == 0 {
			return &Repository{}, fmt.Errorf("state %s not found in gittuf namespace", stateID)
		}
		if len(commitObj.ParentHashes) > 1 {
			return &Repository{}, fmt.Errorf("state %s has multiple parents", iteratorHash.String())
		}

		iteratorHash = commitObj.ParentHashes[0]
	}

	// Now that we've validated it's a valid commit, we can load at that state.
	return loadRepository(repo, stateHash)
}

type Repository struct {
	repository          *git.Repository
	metadataStaging     map[string][]byte // rolename: contents, rolename should NOT include extension
	keysStaging         map[string][]byte // keyID: PubKey
	tip                 plumbing.Hash
	tree                plumbing.Hash
	rootKeys            map[string]object.TreeEntry // keyID: TreeEntry object
	metadataIdentifiers map[string]object.TreeEntry // filename: TreeEntry object
	written             bool
}

func (r *Repository) Tip() string {
	return r.tip.String()
}

func (r *Repository) TipHash() plumbing.Hash {
	return r.tip
}

func (r *Repository) Tree() (*object.Tree, error) {
	return r.repository.TreeObject(r.tree)
}

func (r *Repository) Written() bool {
	return r.written
}

func (r *Repository) GetCommitObject(id string) (*object.Commit, error) {
	return r.GetCommitObjectFromHash(plumbing.NewHash(id))
}

func (r *Repository) GetCommitObjectFromHash(hash plumbing.Hash) (*object.Commit, error) {
	return r.repository.CommitObject(hash)
}

func (r *Repository) GetTreeObject(id string) (*object.Tree, error) {
	return r.GetTreeObjectFromHash(plumbing.NewHash(id))
}

func (r *Repository) GetTreeObjectFromHash(hash plumbing.Hash) (*object.Tree, error) {
	return r.repository.TreeObject(hash)
}

func (r *Repository) GetTreeForNamespace(namespace string) (*object.Tree, error) {
	tree, err := r.repository.TreeObject(r.tree)
	if err != nil {
		return &object.Tree{}, err
	}
	for _, entry := range tree.Entries {
		if entry.Name == namespace {
			return r.GetTreeObjectFromHash(entry.Hash)
		}
	}
	return &object.Tree{}, fmt.Errorf("tree not found for namespace %s", namespace)
}

func (r *Repository) GetMetadataForState(stateID string) (map[string][]byte, error) {
	metadata := map[string][]byte{}

	commit, err := r.GetCommitObject(stateID)
	if err != nil {
		return metadata, err
	}
	tree, err := r.GetTreeObjectFromHash(commit.TreeHash)
	if err != nil {
		return metadata, err
	}

	for _, e := range tree.Entries {
		_, contents, err := readBlob(r.repository, e.Hash)
		if err != nil {
			return map[string][]byte{}, err
		}
		metadata[e.Name] = contents
	}

	return metadata, nil
}

func (r *Repository) HasFile(roleName string) bool {
	_, exists := r.metadataIdentifiers[roleName]
	return exists
}

func (r *Repository) GetCurrentMetadataBytes(roleName string) ([]byte, error) {
	_, contents, err := readBlob(r.repository, r.metadataIdentifiers[roleName].Hash)
	if err != nil {
		return []byte{}, err
	}
	return contents, nil
}

func (r *Repository) GetCurrentMetadataString(roleName string) (string, error) {
	contents, err := r.GetCurrentMetadataBytes(roleName)
	return string(contents), err
}

func (r *Repository) GetAllCurrentMetadata() (map[string][]byte, error) {
	metadata := map[string][]byte{}
	for roleName, treeEntry := range r.metadataIdentifiers {
		_, contents, err := readBlob(r.repository, treeEntry.Hash)
		if err != nil {
			return map[string][]byte{}, err
		}
		metadata[roleName] = contents
	}
	return metadata, nil
}

func (r *Repository) GetRootKey(keyID string) (tufdata.PublicKey, error) {
	var key tufdata.PublicKey
	contents, err := r.GetRootKeyBytes(keyID)
	if err != nil {
		return tufdata.PublicKey{}, err
	}
	err = json.Unmarshal(contents, &key)
	return key, err
}

func (r *Repository) GetRootKeyBytes(keyID string) ([]byte, error) {
	_, contents, err := readBlob(r.repository, r.rootKeys[keyID].Hash)
	return contents, err
}

func (r *Repository) GetRootKeyString(keyID string) (string, error) {
	contents, err := r.GetRootKeyBytes(keyID)
	return string(contents), err
}

func (r *Repository) GetAllRootKeys() (map[string]tufdata.PublicKey, error) {
	keys := map[string]tufdata.PublicKey{}
	for keyID, treeEntry := range r.rootKeys {
		_, contents, err := readBlob(r.repository, treeEntry.Hash)
		if err != nil {
			return map[string]tufdata.PublicKey{}, err
		}

		var key tufdata.PublicKey
		err = json.Unmarshal(contents, &key)
		if err != nil {
			return map[string]tufdata.PublicKey{}, err
		}

		keys[keyID] = key
	}
	return keys, nil
}

func (r *Repository) StageMetadata(roleName string, contents []byte) {
	r.metadataStaging[roleName] = contents
	r.written = false
}

func (r *Repository) StageMetadataAndCommit(roleName string, contents []byte) error {
	r.StageMetadata(roleName, contents)
	return r.Commit()
}

func (r *Repository) StageMultipleMetadata(metadata map[string][]byte) {
	for roleName, contents := range metadata {
		r.StageMetadata(roleName, contents)
	}
}

func (r *Repository) StageAndCommitMultipleMetadata(metadata map[string][]byte) error {
	r.StageMultipleMetadata(metadata)
	return r.Commit()
}

func (r *Repository) StageKey(key tufdata.PublicKey) error {
	r.written = false
	contents, err := json.Marshal(key)
	if err != nil {
		return err
	}
	keyIDs := key.IDs()
	for _, keyID := range keyIDs {
		r.keysStaging[keyID] = contents
	}
	return nil
}

func (r *Repository) StageKeyAndCommit(key tufdata.PublicKey) error {
	err := r.StageKey(key)
	if err != nil {
		return err
	}
	return r.Commit()
}

func (r *Repository) StageKeys(keys []tufdata.PublicKey) error {
	for _, key := range keys {
		err := r.StageKey(key)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) StageKeysAndCommit(keys []tufdata.PublicKey) error {
	err := r.StageKeys(keys)
	if err != nil {
		return err
	}
	return r.Commit()
}

func (r *Repository) Commit() error {
	if r.Written() {
		// Nothing to do
		return nil
	}

	// We need to create a new tree that includes unchanged entries and the
	// newly staged metadata.
	metadataEntries := []object.TreeEntry{}
	for roleName, treeEntry := range r.metadataIdentifiers {
		if _, exists := r.metadataStaging[roleName]; exists {
			// We'll not reuse the entries for staged metadata
			continue
		}
		metadataEntries = append(metadataEntries, treeEntry)
	}

	// Write staged blobs and add them to currentEntries
	for roleName, contents := range r.metadataStaging {
		identifier, err := writeBlob(r.repository, contents)
		if err != nil {
			return err
		}
		treeEntry := object.TreeEntry{
			Name: fmt.Sprintf("%s.json", roleName),
			Mode: filemode.Regular,
			Hash: identifier,
		}
		r.metadataIdentifiers[roleName] = treeEntry
		metadataEntries = append(metadataEntries, treeEntry)
	}

	// Create a new tree object
	metadataTreeHash, err := writeTree(r.repository, metadataEntries)
	if err != nil {
		return err
	}

	// FIXME: DRY?
	rootKeyEntries := []object.TreeEntry{}
	for keyID, treeEntry := range r.rootKeys {
		if _, exists := r.keysStaging[keyID]; exists {
			continue
		}
		rootKeyEntries = append(rootKeyEntries, treeEntry)
	}

	for keyID, contents := range r.keysStaging {
		identifier, err := writeBlob(r.repository, contents)
		if err != nil {
			return err
		}
		treeEntry := object.TreeEntry{
			Name: fmt.Sprintf("%s.pub", keyID),
			Mode: filemode.Regular,
			Hash: identifier,
		}
		r.rootKeys[keyID] = treeEntry
		rootKeyEntries = append(rootKeyEntries, treeEntry)
	}

	keysTreeHash, err := writeTree(r.repository, rootKeyEntries)
	if err != nil {
		return err
	}

	topLevelEntries := []object.TreeEntry{}
	topLevelEntries = append(topLevelEntries, object.TreeEntry{
		Name: MetadataDir,
		Mode: filemode.Dir,
		Hash: metadataTreeHash,
	})
	topLevelEntries = append(topLevelEntries, object.TreeEntry{
		Name: KeysDir,
		Mode: filemode.Dir,
		Hash: keysTreeHash,
	})

	treeHash, err := writeTree(r.repository, topLevelEntries)
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

func (r *Repository) RemoveMetadata(roleNames []string) error {
	r.written = false

	for _, role := range roleNames {
		delete(r.metadataStaging, role)
		delete(r.metadataIdentifiers, role)
	}

	return r.Commit()
}

func (r *Repository) RemoveKeys(keyIDs []string) error {
	r.written = false

	for _, keyID := range keyIDs {
		delete(r.keysStaging, keyID)
		delete(r.rootKeys, keyID)
	}
	return r.Commit()
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
