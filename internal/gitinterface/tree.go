// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path"
	"sort"
	"strings"

	"github.com/gittuf/gittuf/internal/dev"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
)

// WriteTree creates a Git tree with the specified entries. It sorts the entries
// prior to creating the tree.
func WriteTree(repo *git.Repository, entries []object.TreeEntry) (plumbing.Hash, error) {
	sort.Slice(entries, func(i int, j int) bool {
		return entries[i].Name < entries[j].Name
	})
	obj := repo.Storer.NewEncodedObject()
	tree := object.Tree{
		Entries: entries,
	}
	if err := tree.Encode(obj); err != nil {
		return plumbing.ZeroHash, err
	}
	return repo.Storer.SetEncodedObject(obj)
}

// GetTree returns the requested tree object.
func GetTree(repo *git.Repository, treeID plumbing.Hash) (*object.Tree, error) {
	return repo.TreeObject(treeID)
}

// EmptyTree returns the hash of an empty tree in a Git repository.
// Note: it is generated on the fly rather than stored as a constant to support
// SHA-256 repositories in future.
func EmptyTree() plumbing.Hash {
	obj := memory.NewStorage().NewEncodedObject()
	tree := object.Tree{}
	tree.Encode(obj) //nolint:errcheck

	return obj.Hash()
}

func (r *Repository) EmptyTree() (Hash, error) {
	treeID, err := r.executor("hash-object", "-t", "tree", "--stdin").execute()
	if err != nil {
		return ZeroHash, fmt.Errorf("unable to hash empty tree: %w", err)
	}

	hash, err := NewHash(treeID)
	if err != nil {
		return ZeroHash, fmt.Errorf("empty tree has invalid Git ID: %w", err)
	}

	return hash, nil
}

// GetAllFilesInTree returns all filepaths and the corresponding hash in the
// specified tree.
func GetAllFilesInTree(tree *object.Tree) (map[string]plumbing.Hash, error) {
	treeWalker := object.NewTreeWalker(tree, true, nil)
	defer treeWalker.Close()

	files := map[string]plumbing.Hash{}

	for {
		// This is based on FilesIter in go-git. That implementation loads each
		// blob which we don't want to do.
		name, entry, err := treeWalker.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}

		if entry.Mode == filemode.Dir || entry.Mode == filemode.Submodule {
			continue
		}

		files[name] = entry.Hash
	}

	return files, nil
}

// GetAllFilesInTree returns all filepaths and the corresponding blob hashes in
// the specified tree.
func (r *Repository) GetAllFilesInTree(treeID Hash) (map[string]Hash, error) {
	stdOut, err := r.executor("ls-tree", "-r", "--format=%(path) %(objectname)", treeID.String()).execute()
	if err != nil {
		return nil, fmt.Errorf("unable to enumerate all files in tree: %w", err)
	}

	if stdOut == "" {
		return nil, nil // alternatively, just check if treeID is empty tree?
	}

	entries := strings.Split(stdOut, "\n")
	if len(entries) == 0 {
		return nil, nil
	}

	files := map[string]Hash{}
	for _, entry := range entries {
		// we control entry's format in --format above, so no need to check
		// length of split
		entrySplit := strings.Split(entry, " ")

		hash, err := NewHash(entrySplit[1])
		if err != nil {
			return nil, fmt.Errorf("invalid Git ID '%s' for path '%s': %w", entrySplit[1], entrySplit[0], err)
		}

		files[entrySplit[0]] = hash
	}

	return files, nil
}

// GetMergeTree computes the merge tree for the commits passed in. The tree is
// not written to the object store. Assuming a typical merge workflow, the first
// commit is expected to be the tip of the base branch. As such, the second
// commit is expected to be merged into the first. If the first commit is zero,
// the second commit's tree is returned.
func GetMergeTree(repo *git.Repository, commitAID, commitBID string) (string, error) {
	if !dev.InDevMode() {
		return "", dev.ErrNotInDevMode
	}

	// Base branch commit ID is zero
	if plumbing.NewHash(commitAID).IsZero() {
		// Return commitB's tree
		commitB, err := GetCommit(repo, plumbing.NewHash(commitBID))
		if err != nil {
			return "", err
		}

		return commitB.TreeHash.String(), nil
	}

	// go-git does not support three way merges
	command := exec.Command("git", "merge-tree", commitAID, commitBID) //nolint:gosec
	stdOut, err := command.Output()
	if err != nil {
		return "", err
	}

	stdOutString := strings.TrimSpace(string(stdOut))
	return stdOutString, nil
}

// GetMergeTree computes the merge tree for the commits passed in. The tree is
// not written to the object store. Assuming a typical merge workflow, the first
// commit is expected to be the tip of the base branch. As such, the second
// commit is expected to be merged into the first. If the first commit is zero,
// the second commit's tree is returned.
func (r *Repository) GetMergeTree(commitAID, commitBID Hash) (Hash, error) {
	if err := r.ensureIsCommit(commitAID); err != nil {
		return ZeroHash, err
	}
	if err := r.ensureIsCommit(commitBID); err != nil {
		return ZeroHash, err
	}

	if commitAID.IsZero() {
		return r.GetCommitTreeID(commitBID)
	}

	stdOut, err := r.executor("merge-tree", commitAID.String(), commitBID.String()).execute()
	if err != nil {
		return ZeroHash, fmt.Errorf("unable to compute merge tree: %w", err)
	}

	treeHash, err := NewHash(stdOut)
	if err != nil {
		return ZeroHash, fmt.Errorf("invalid merge tree ID: %w", err)
	}

	return treeHash, nil
}

// TreeBuilder is used to create multi-level trees in a repository.
// Based on `buildTreeHelper` in go-git.
type TreeBuilder struct {
	r       *git.Repository
	trees   map[string]*object.Tree
	entries map[string]*object.TreeEntry
}

// NewTreeBuilder returns a TreeBuilder instance for the repository.
func NewTreeBuilder(repo *git.Repository) *TreeBuilder {
	return &TreeBuilder{r: repo}
}

// WriteRootTreeFromBlobIDs accepts a map of paths to their blob IDs and returns
// the root tree ID that contains these files.
func (t *TreeBuilder) WriteRootTreeFromBlobIDs(files map[string]plumbing.Hash) (plumbing.Hash, error) {
	rootNodeKey := ""
	t.trees = map[string]*object.Tree{rootNodeKey: {}}
	t.entries = map[string]*object.TreeEntry{}

	for path, blobID := range files {
		t.buildIntermediates(path, blobID)
	}

	return t.writeTrees(rootNodeKey, t.trees[rootNodeKey])
}

// buildIntermediates identifies the intermediate trees that must be constructed
// for the specified path.
func (t *TreeBuilder) buildIntermediates(name string, blobID plumbing.Hash) {
	parts := strings.Split(name, "/")

	var fullPath string
	for _, part := range parts {
		parent := fullPath
		fullPath = path.Join(fullPath, part)

		t.buildTree(name, parent, fullPath, blobID)
	}
}

// buildTree populates tree and entry information for each tree that must be
// created.
func (t *TreeBuilder) buildTree(name, parent, fullPath string, blobID plumbing.Hash) {
	if _, ok := t.trees[fullPath]; ok {
		return
	}

	if _, ok := t.entries[fullPath]; ok {
		return
	}

	entry := object.TreeEntry{Name: path.Base(fullPath)}

	if fullPath == name {
		entry.Mode = filemode.Regular
		entry.Hash = blobID
	} else {
		entry.Mode = filemode.Dir
		t.trees[fullPath] = &object.Tree{}
	}

	t.trees[parent].Entries = append(t.trees[parent].Entries, entry)
}

// writeTrees recursively stores each tree that must be created in the
// repository's object store. It returns the ID of the tree created at each
// invocation.
func (t *TreeBuilder) writeTrees(parent string, tree *object.Tree) (plumbing.Hash, error) {
	for i, e := range tree.Entries {
		if e.Mode != filemode.Dir && !e.Hash.IsZero() {
			continue
		}

		p := path.Join(parent, e.Name)
		entryID, err := t.writeTrees(p, t.trees[p])
		if err != nil {
			return plumbing.ZeroHash, err
		}
		e.Hash = entryID

		tree.Entries[i] = e
	}

	return WriteTree(t.r, tree.Entries)
}

// ReplacementTreeBuilder is used to create multi-level trees in a repository.
// Based on `buildTreeHelper` in go-git.
// This will replace TreeBuilder above, and will be renamed at that point.
type ReplacementTreeBuilder struct {
	repo    *Repository
	trees   map[string]*entry
	entries map[string]*entry
}

func NewReplacementTreeBuilder(repo *Repository) *ReplacementTreeBuilder {
	return &ReplacementTreeBuilder{repo: repo}
}

// WriteRootTreeFromBlobIDs accepts a map of paths to their blob IDs and returns
// the root tree ID that contains these files.
func (t *ReplacementTreeBuilder) WriteRootTreeFromBlobIDs(files map[string]Hash) (Hash, error) {
	rootNoteKey := ""
	t.trees = map[string]*entry{rootNoteKey: {}}
	t.entries = map[string]*entry{}

	for path, gitID := range files {
		t.buildIntermediates(path, gitID)
	}

	return t.writeTrees(rootNoteKey, t.trees[rootNoteKey])
}

// buildIntermediates identifies the intermediate trees that must be constructed
// for the specified path.
func (t *ReplacementTreeBuilder) buildIntermediates(name string, gitID Hash) {
	parts := strings.Split(name, "/")

	var fullPath string
	for _, part := range parts {
		parent := fullPath
		fullPath = path.Join(fullPath, part)

		t.buildTree(name, parent, fullPath, gitID)
	}
}

// buildTree populates tree and entry information for each tree that must be
// created.
func (t *ReplacementTreeBuilder) buildTree(name, parent, fullPath string, gitID Hash) {
	if _, ok := t.trees[fullPath]; ok {
		return
	}

	if _, ok := t.entries[fullPath]; ok {
		return
	}

	entryObj := &entry{name: path.Base(fullPath), gitID: ZeroHash}

	if fullPath == name {
		entryObj.isDir = false
		entryObj.gitID = gitID
	} else {
		entryObj.isDir = true
		t.trees[fullPath] = &entry{}
	}

	t.trees[parent].entries = append(t.trees[parent].entries, entryObj)
}

// writeTrees recursively stores each tree that must be created in the
// repository's object store. It returns the ID of the tree created at each
// invocation.
func (t *ReplacementTreeBuilder) writeTrees(parent string, tree *entry) (Hash, error) {
	for i, e := range tree.entries {
		if !e.isDir && !e.gitID.IsZero() {
			continue
		}

		p := path.Join(parent, e.name)
		entryID, err := t.writeTrees(p, t.trees[p])
		if err != nil {
			return ZeroHash, err
		}
		e.gitID = entryID

		tree.entries[i] = e
	}

	return t.writeTree(tree.entries)
}

// writeTree creates a tree in the repository for the specified entries. It
// only supports a typical blob with permission 0o644 and a subtree. This is
// because it is only intended for use with gittuf specific metadata and tests.
// Generic tree creation is left to invocations of the Git binary by the user.
func (t *ReplacementTreeBuilder) writeTree(entries []*entry) (Hash, error) {
	input := ""
	for _, entry := range entries {
		// this is very opinionated about the modes right now because the plan
		// is to use it for gittuf metadata, which requires regular files and
		// subdirectories
		if entry.isDir {
			input += "040000 tree " + entry.gitID.String() + "\t" + entry.name
		} else {
			input += "100644 blob " + entry.gitID.String() + "\t" + entry.name
		}
		input += "\n"
	}

	stdOut, err := t.repo.executor("mktree").withStdIn(bytes.NewBufferString(input)).execute()
	if err != nil {
		return ZeroHash, fmt.Errorf("unable to write Git tree: %w", err)
	}

	treeID, err := NewHash(stdOut)
	if err != nil {
		return ZeroHash, fmt.Errorf("invalid tree ID: %w", err)
	}

	return treeID, nil
}

// entry is a helper type that represents an entry in a Git tree. If `isDir` is
// true, it indicates the entry represents a subtree.
type entry struct {
	name    string
	isDir   bool
	gitID   Hash
	entries []*entry // only used when isDir is true
}
