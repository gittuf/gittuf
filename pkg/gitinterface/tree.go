// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"bytes"
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/gittuf/gittuf/pkg/gitstore"
)

var (
	ErrTreeDoesNotHavePath             = errors.New("tree does not have requested path")
	ErrCopyingBlobIDsDoNotMatch        = errors.New("blob ID in local repository does not match upstream repository")
	ErrCannotCreateSubtreeIntoRootTree = errors.New("subtree path target cannot be empty or root of tree")
)

func (r *Repository) EmptyTree() (Hash, error) {
	treeID, err := r.executor("hash-object", "-t", "tree", "--stdin").executeString()
	if err != nil {
		return ZeroHash, fmt.Errorf("unable to hash empty tree: %w", err)
	}

	hash, err := NewHash(treeID)
	if err != nil {
		return ZeroHash, fmt.Errorf("empty tree has invalid Git ID: %w", err)
	}

	return hash, nil
}

// GetPathIDInTree returns the Git ID pointed to by the path in the specified
// tree if the path exists. If not, a corresponding error is returned.  For
// example, if the tree contains a single blob `foo/bar/baz`, querying the ID
// for `foo/bar/baz` will return the blob ID for baz. Querying the ID for
// `foo/bar` will return the intermediate tree ID for bar, while querying for
// `foo/baz` will return an error.
func (r *Repository) GetPathIDInTree(treeID Hash, treePath string) (Hash, error) {
	treePath = strings.TrimSuffix(treePath, "/")
	components := strings.Split(treePath, "/")

	currentTreeID := treeID
	for len(components) != 0 {
		entries, err := r.GetEntriesInTree(currentTreeID)
		if err != nil {
			return nil, err
		}

		entryID, has := findEntryID(entries, components[0])
		if !has {
			return nil, fmt.Errorf("%w: %s", ErrTreeDoesNotHavePath, treePath)
		}

		currentTreeID = entryID
		components = components[1:]
	}

	return currentTreeID, nil
}

func findEntryID(entries []TreeEntry, name string) (Hash, bool) {
	for _, entry := range entries {
		if entry.Path == name {
			return entry.ID, true
		}
	}
	return nil, false
}

// GetEntriesInTree returns the immediate entries of the specified Git tree
// (without recursively expanding subtrees). Each entry carries its name, ID,
// and kind.
func (r *Repository) GetEntriesInTree(treeID Hash) ([]TreeEntry, error) {
	// From Git 2.36, we can use --format here. However, it appears a not
	// insignificant number of developers are still on Git 2.34.1, a side effect
	// of being on Ubuntu 22.04. 22.04 is still widely used in WSL2 environments.
	// So, we're removing --format and parsing the output differently to handle
	// the extra information for each entry we don't need.
	stdOut, err := r.executor("ls-tree", treeID.String()).executeString()
	if err != nil {
		return nil, fmt.Errorf("unable to enumerate items in tree '%s': %w", treeID.String(), err)
	}

	if stdOut == "" {
		return nil, nil // alternatively, just check if treeID is empty tree?
	}

	lines := strings.Split(stdOut, "\n")
	entries := make([]TreeEntry, 0, len(lines))
	for _, line := range lines {
		// Without --format, the output is in the following format:
		// <mode> SP <type> SP <object> TAB <file>
		// From: https://git-scm.com/docs/git-ls-tree/2.34.1#_output_format

		fields := strings.Split(line, " ")
		// fields[0] is <mode> -- discard
		// fields[1] is <type> -- blob or tree
		// fields[2] is <object> TAB <file>
		objectAndName := strings.Split(fields[2], "\t")

		hash, err := NewHash(objectAndName[0])
		if err != nil {
			return nil, fmt.Errorf("invalid Git ID '%s' for path '%s': %w", objectAndName[0], objectAndName[1], err)
		}

		kind := gitstore.KindBlob
		if fields[1] == "tree" {
			kind = gitstore.KindSubtree
		}

		entries = append(entries, TreeEntry{Path: objectAndName[1], ID: hash, Kind: kind})
	}

	return entries, nil
}

// GetAllFilesInTree returns all filepaths and the corresponding blob hashes in
// the specified tree.
func (r *Repository) GetAllFilesInTree(treeID Hash) (map[string]Hash, error) {
	// From Git 2.36, we can use --format here. However, it appears a not
	// insignificant number of developers are still on Git 2.34.1, a side effect
	// of being on Ubuntu 22.04. 22.04 is still widely used in WSL2 environments.
	// So, we're removing --format and parsing the output differently to handle
	// the extra information for each entry we don't need.
	stdOut, err := r.executor("ls-tree", "-r", treeID.String()).executeString()
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
		// Without --format, the output is in the following format:
		// <mode> SP <type> SP <object> TAB <file>
		// From: https://git-scm.com/docs/git-ls-tree/2.34.1#_output_format

		entrySplit := strings.Split(entry, " ")
		// entrySplit[0] is <mode> -- discard
		// entrySplit[1] is <type> -- discard
		// entrySplit[2] is <object> TAB <file> -- keep
		entrySplit = strings.Split(entrySplit[2], "\t")

		// <object> is really the object ID
		hash, err := NewHash(entrySplit[0])
		if err != nil {
			return nil, fmt.Errorf("invalid Git ID '%s' for path '%s': %w", entrySplit[0], entrySplit[1], err)
		}

		files[entrySplit[1]] = hash
	}

	return files, nil
}

// GetMergeTree computes the merge tree for the commits passed in. The tree is
// not written to the object store. Assuming a typical merge workflow, the first
// commit is expected to be the tip of the base branch. As such, the second
// commit is expected to be merged into the first. If the first commit is zero,
// the second commit's tree is returned.
func (r *Repository) GetMergeTree(commitAID, commitBID Hash) (Hash, error) {
	if err := r.ensureIsCommit(commitBID); err != nil {
		return ZeroHash, err
	}

	if commitAID.IsZero() {
		// fast-forward merge -> use tree ID from commitB
		return r.GetCommitTreeID(commitBID)
	}

	// Only commitB needs to be non-zero, we can allow fast-forward merges when
	// the base commit is zero. So, check this only after above
	if err := r.ensureIsCommit(commitAID); err != nil {
		return ZeroHash, err
	}

	stdOut, err := r.executor("merge-tree", commitAID.String(), commitBID.String()).executeString()
	if err != nil {
		return ZeroHash, fmt.Errorf("unable to compute merge tree: %w", err)
	}

	treeHash, err := NewHash(stdOut)
	if err != nil {
		return ZeroHash, fmt.Errorf("invalid merge tree ID: %w", err)
	}

	return treeHash, nil
}

// CreateSubtreeFromUpstreamRepository accepts an upstream repository handler
// and a commit ID in the upstream repository. This information is used to copy
// the entire contents of the commit's Git tree into the specified localPath in
// the localRef. A new commit is added to localRef with the changes made to
// localPath. localPath represents a directory path where the changes are copied
// to. Existing items in that directory are overwritten in the subsequently
// created commit in localRef. localPath must be specified, if left blank (say
// to imply copying into the root directory of the downstream repository),
// creating a subtree will fail.
func (r *Repository) CreateSubtreeFromUpstreamRepository(upstream *Repository, upstreamCommitID Hash, upstreamPath, localRef, localPath string) (Hash, error) {
	if localPath == "" {
		return nil, ErrCannotCreateSubtreeIntoRootTree
	}
	currentTip, err := r.GetReference(localRef)
	if err != nil {
		if !errors.Is(err, ErrReferenceNotFound) {
			return nil, err
		}
	}

	entries := []TreeEntry{}
	if !currentTip.IsZero() {
		currentRefTree, err := r.GetCommitTreeID(currentTip)
		if err != nil {
			return nil, err
		}
		currentFiles, err := r.GetAllFilesInTree(currentRefTree)
		if err != nil {
			return nil, err
		}

		// Ignore entries for `localPath` to account for upstream deletions
		// If localPath is foo/, we want to ignore all items under foo/
		// If localPath is foo, we want to ignore all items under foo/
		// If localPath is foo, we DO NOT want to remove all items under foobar/
		// So, add the / suffix if necessary to localPath
		if !strings.HasSuffix(localPath, "/") {
			localPath += "/"
		}

		// Create list of TreeEntry objects representing all blobs except those
		// currently under localPath
		for filePath, blobID := range currentFiles {
			if !strings.HasPrefix(filePath, localPath) {
				entries = append(entries, NewEntryBlob(filePath, blobID))
			}
		}
	}

	// Remove trailing "/" now
	localPath = strings.TrimSuffix(localPath, "/")

	treeID, err := upstream.GetCommitTreeID(upstreamCommitID)
	if err != nil {
		return nil, err
	}

	if upstreamPath != "" {
		// If upstreamPath is empty, then the entire tree is copied over,
		// otherwise, identify the subtree to copy over
		treeID, err = upstream.GetPathIDInTree(treeID, upstreamPath)
		if err != nil {
			return nil, err
		}
	}

	if r.HasObject(treeID) {
		// Use existing intermediate tree
		entries = append(entries, NewEntryTree(localPath, treeID))
	} else {
		// We have to create the intermediate tree for localPath
		filesToCopy, err := upstream.GetAllFilesInTree(treeID)
		if err != nil {
			return nil, err
		}

		for blobPath, blobID := range filesToCopy {
			// if blob already exists, we don't need to carry out expensive
			// read/write
			if !r.HasObject(blobID) {
				blob, err := upstream.ReadBlob(blobID)
				if err != nil {
					return nil, err
				}
				localBlobID, err := r.WriteBlob(blob)
				if err != nil {
					return nil, err
				}
				if !localBlobID.Equal(blobID) {
					return nil, ErrCopyingBlobIDsDoNotMatch
				}
			}

			// add blob to entries, with the path including the localPath prefix
			entries = append(entries, NewEntryBlob(path.Join(localPath, blobPath), blobID))
		}
	}

	builder := NewTreeBuilder(r)
	newTreeID, err := builder.WriteTreeFromEntries(entries)
	if err != nil {
		return nil, err
	}

	commitID, err := r.Commit(newTreeID, localRef, fmt.Sprintf("Update contents of '%s'\n", localPath), false)
	if err != nil {
		return nil, err
	}

	worktree, err := r.GetWorktree()
	if err != nil {
		// bare repositories have no worktree to update
		if !errors.Is(err, ErrNoWorktree) {
			return nil, err
		}
	} else {
		head, err := r.GetSymbolicReferenceTarget("HEAD")
		if err != nil {
			return nil, err
		}
		if head == localRef {
			if _, err := r.executor("restore", "--staged", "--", localPath).withDir(worktree).executeString(); err != nil {
				return nil, err
			}
			if _, err := r.executor("restore", "--", localPath).withDir(worktree).executeString(); err != nil {
				return nil, err
			}
		}
	}

	return commitID, nil
}

// WriteTree writes a Git tree from the given entries and returns its ID.
// Intermediate trees implied by "/" in an entry's path are created
// automatically. The result is independent of entry order because git mktree
// normalizes entries. It returns gitstore.ErrDuplicateTreePath if two entries
// share a path.
func (r *Repository) WriteTree(entries []TreeEntry) (Hash, error) {
	seen := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		if _, ok := seen[entry.Path]; ok {
			return ZeroHash, fmt.Errorf("%w: %s", gitstore.ErrDuplicateTreePath, entry.Path)
		}
		seen[entry.Path] = struct{}{}
	}

	return NewTreeBuilder(r).WriteTreeFromEntries(entries)
}

// TreeBuilder creates multi-level trees in a repository. Based on
// `buildTreeHelper` in go-git.
type TreeBuilder struct {
	repo  *Repository
	trees map[string]*entryTree
}

func NewTreeBuilder(repo *Repository) *TreeBuilder {
	return &TreeBuilder{repo: repo}
}

// WriteTreeFromEntries writes the tree containing the given entries, building
// any intermediate trees their paths require, and returns its ID.
func (t *TreeBuilder) WriteTreeFromEntries(entries []TreeEntry) (Hash, error) {
	rootNodeKey := ""
	t.trees = map[string]*entryTree{rootNodeKey: {}}

	for _, entry := range entries {
		t.identifyIntermediates(entry)
	}

	return t.writeTrees(rootNodeKey, t.trees[rootNodeKey])
}

// identifyIntermediates identifies the intermediate trees that must be
// constructed for the specified path.
func (t *TreeBuilder) identifyIntermediates(entry TreeEntry) {
	parts := strings.Split(entry.Path, "/")

	var fullPath string
	for _, part := range parts {
		parent := fullPath
		fullPath = path.Join(fullPath, part)

		t.populateTree(parent, fullPath, entry)
	}
}

// populateTree populates tree and entry information for each tree that must be
// created.
func (t *TreeBuilder) populateTree(parent, fullPath string, entry TreeEntry) {
	if _, ok := t.trees[fullPath]; ok {
		return
	}

	var node treeNode

	if fullPath == entry.Path {
		// => This is a leaf node. Its kind is authoritative: a subtree grafts
		// an existing tree object, a blob is a regular file.
		if entry.Kind == gitstore.KindSubtree {
			node = &entryTree{
				name:          path.Base(fullPath),
				gitID:         entry.ID,
				alreadyExists: true,
			}
		} else {
			node = &entryBlob{
				name:  path.Base(fullPath),
				gitID: entry.ID,
			}
		}
	} else {
		// => This is an intermediate node, has to be a tree that we must build
		node = &entryTree{
			name:          path.Base(fullPath),
			gitID:         ZeroHash,
			alreadyExists: false,
		}
		t.trees[fullPath] = &entryTree{}
	}

	t.trees[parent].entries = append(t.trees[parent].entries, node)
}

// writeTrees recursively stores each tree that must be created in the
// repository's object store. It returns the ID of the tree created at each
// invocation.
func (t *TreeBuilder) writeTrees(parent string, tree *entryTree) (Hash, error) {
	for i, e := range tree.entries {
		switch e := e.(type) {
		case *entryTree:
			if e.alreadyExists {
				// The tree already exists and we don't need to write it again.
				continue
			}

			p := path.Join(parent, e.name)
			entryID, err := t.writeTrees(p, t.trees[p])
			if err != nil {
				return ZeroHash, err
			}
			e.gitID = entryID

			tree.entries[i] = e

		case *entryBlob:
			continue
		}
	}

	return t.writeTree(tree.entries)
}

// writeTree creates a tree in the repository for the specified entries. It
// only supports a typical blob with permission 0o644 and a subtree. This is
// because it is only intended for use with gittuf specific metadata and tests.
// Generic tree creation is left to invocations of the Git binary by the user.
func (t *TreeBuilder) writeTree(entries []treeNode) (Hash, error) {
	input := ""
	for _, entry := range entries {
		// this is very opinionated about the modes right now because the plan
		// is to use it for gittuf metadata, which requires regular files and
		// subdirectories
		switch entry := entry.(type) {
		case *entryTree:
			input += "040000 tree " + entry.gitID.String() + "\t" + entry.name
		case *entryBlob:
			// TODO: support entryBlob's permissions here
			input += "100644 blob " + entry.gitID.String() + "\t" + entry.name
		}
		input += "\n"
	}

	stdOut, err := t.repo.executor("mktree").withStdIn(bytes.NewBufferString(input)).executeString()
	if err != nil {
		return ZeroHash, fmt.Errorf("unable to write Git tree: %w", err)
	}

	treeID, err := NewHash(stdOut)
	if err != nil {
		return ZeroHash, fmt.Errorf("invalid tree ID: %w", err)
	}

	return treeID, nil
}

// TreeEntry describes one entry to place in a tree. It aliases
// gitstore.TreeEntry so both the Storer interface and gitinterface name the
// same type.
type TreeEntry = gitstore.TreeEntry

// treeNode is the builder's internal representation of a resolved tree entry,
// either a (possibly to-be-created) subtree or a blob.
type treeNode interface {
	getName() string
	getID() Hash
}

// entryTree implements treeNode and indicates the entry is for a Git tree.
type entryTree struct {
	name          string
	gitID         Hash
	alreadyExists bool
	entries       []treeNode
}

func (e *entryTree) getName() string {
	return e.name
}

func (e *entryTree) getID() Hash {
	return e.gitID
}

// NewEntryTree creates a TreeEntry that grafts an existing Git tree at name.
func NewEntryTree(name string, gitID Hash) TreeEntry {
	return TreeEntry{Path: name, ID: gitID, Kind: gitstore.KindSubtree}
}

// entryBlob implements treeNode and indicates the entry is for a Git blob.
type entryBlob struct {
	name  string
	gitID Hash
}

func (e *entryBlob) getName() string {
	return e.name
}

func (e *entryBlob) getID() Hash {
	return e.gitID
}

// NewEntryBlob creates a TreeEntry that represents a Git blob.
func NewEntryBlob(name string, gitID Hash) TreeEntry {
	return TreeEntry{Path: name, ID: gitID, Kind: gitstore.KindBlob}
}

// ensureIsTree is a helper to check that the ID represents a Git tree
// object.
func (r *Repository) ensureIsTree(treeID Hash) error {
	objType, err := r.executor("cat-file", "-t", treeID.String()).executeString()
	if err != nil {
		return fmt.Errorf("unable to inspect if object is tree: %w", err)
	} else if objType != "tree" {
		return fmt.Errorf("requested Git ID '%s' is not a tree object", treeID.String())
	}

	return nil
}
