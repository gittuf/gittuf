// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
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
func (r *Repository) GetPathIDInTree(treePath string, treeID Hash) (Hash, error) {
	treePath = strings.TrimSuffix(treePath, "/")
	components := strings.Split(treePath, "/")

	currentTreeID := treeID
	for len(components) != 0 {
		items, err := r.GetTreeItems(currentTreeID)
		if err != nil {
			return nil, err
		}

		entryID, has := items[components[0]]
		if !has {
			return nil, fmt.Errorf("%w: %s", ErrTreeDoesNotHavePath, treePath)
		}

		currentTreeID = entryID
		components = components[1:]
	}

	return currentTreeID, nil
}

// GetTreeItems returns the items in a specified Git tree without recursively
// expanding subtrees.
func (r *Repository) GetTreeItems(treeID Hash) (map[string]Hash, error) {
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

	entries := strings.Split(stdOut, "\n")
	if len(entries) == 0 {
		return nil, nil
	}

	items := map[string]Hash{}
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

		items[entrySplit[1]] = hash
	}

	return items, nil
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

	niceGit, err := isNiceGitVersion()
	if err != nil {
		return ZeroHash, err
	}

	var stdOut string
	if !niceGit {
		// Older Git versions do not support merge-tree, and, as such, require
		// quite a long workaround to find what the merge tree is. This
		// workaround boils down to:
		// Create new branch > Merge into said branch > Extract tree hash
		currentBranch, err := r.executor("branch", "--show-current").executeString()
		if err != nil {
			return ZeroHash, fmt.Errorf("unable to determine current branch: %w", err)
		}

		if currentBranch == "" {
			return ZeroHash, fmt.Errorf("currently in detached HEAD state, please switch to a branch")
		}

		_, err = r.executor("checkout", commitAID.String()).executeString()
		if err != nil {
			return ZeroHash, fmt.Errorf("unable to enter detached HEAD state: %w", err)
		}

		_, err = r.executor("merge", "-m", "Computing merge tree", commitBID.String()).executeString()
		if err != nil {
			// Attempt to abort the merge in all cases as a failsafe
			_, abrtErr := r.executor("merge", "--abort").executeString()
			if abrtErr != nil {
				return ZeroHash, fmt.Errorf("unable to perform merge, and unable to abort merge: %w, %w", err, abrtErr)
			}

			return ZeroHash, fmt.Errorf("unable to perform merge: %w", err)
		}

		stdOut, err = r.executor("show", "-s", "--format=%T").executeString()
		if err != nil {
			return ZeroHash, fmt.Errorf("unable to extract tree hash of merge commit: %w", err)
		}

		// Switch back to the branch the user was on
		_, err = r.executor("checkout", currentBranch).executeString()
		if err != nil {
			return ZeroHash, fmt.Errorf("unable to switch back to original branch: %w", err)
		}
	} else {
		stdOut, err = r.executor("merge-tree", commitAID.String(), commitBID.String()).executeString()
		if err != nil {
			return ZeroHash, fmt.Errorf("unable to compute merge tree: %w", err)
		}
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
		treeID, err = upstream.GetPathIDInTree(upstreamPath, treeID)
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

	treeBuilder := NewTreeBuilder(r)
	newTreeID, err := treeBuilder.WriteTreeFromEntries(entries)
	if err != nil {
		return nil, err
	}

	commitID, err := r.Commit(newTreeID, localRef, fmt.Sprintf("Update contents of '%s'\n", localPath), false)
	if err != nil {
		return nil, err
	}

	if !r.IsBare() {
		head, err := r.GetSymbolicReferenceTarget("HEAD")
		if err != nil {
			return nil, err
		}
		if head == localRef {
			worktree := strings.TrimSuffix(r.gitDirPath, ".git") // TODO: this doesn't support detached git dir
			cwd, err := os.Getwd()
			if err != nil {
				return nil, err
			}
			if err := os.Chdir(worktree); err != nil {
				return nil, err
			}
			defer os.Chdir(cwd) //nolint:errcheck

			if _, err := r.executor("restore", "--staged", localPath).executeString(); err != nil {
				return nil, err
			}
			if _, err := r.executor("restore", localPath).executeString(); err != nil {
				return nil, err
			}
		}
	}

	return commitID, nil
}

// TreeBuilder is used to create multi-level trees in a repository.  Based on
// `buildTreeHelper` in go-git.
type TreeBuilder struct {
	repo    *Repository
	trees   map[string]*entryTree
	entries map[string]TreeEntry
}

func NewTreeBuilder(repo *Repository) *TreeBuilder {
	return &TreeBuilder{repo: repo}
}

// WriteTreeFromEntries accepts list of TreeEntry representations, and returns
// the Git ID of the tree that contains these entries. It constructs the
// required intermediate trees.
func (t *TreeBuilder) WriteTreeFromEntries(files []TreeEntry) (Hash, error) {
	rootNodeKey := ""
	t.trees = map[string]*entryTree{rootNodeKey: {}}
	t.entries = map[string]TreeEntry{}

	for _, entry := range files {
		t.identifyIntermediates(entry)
	}

	return t.writeTrees(rootNodeKey, t.trees[rootNodeKey])
}

// identifyIntermediates identifies the intermediate trees that must be
// constructed for the specified path.
func (t *TreeBuilder) identifyIntermediates(entry TreeEntry) {
	parts := strings.Split(entry.getName(), "/")

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

	if _, ok := t.entries[fullPath]; ok {
		return
	}

	var entryObj TreeEntry

	if fullPath == entry.getName() {
		// => This is a leaf node
		// However, gitID _may_ be a tree ID, and we've inserted an existing
		// tree object as a subtree here, we want to support this so that we
		// don't have to recreate trees that already exist

		if err := t.repo.ensureIsTree(entry.getID()); err == nil {
			// gitID represents tree
			entryObj = &entryTree{
				name:          path.Base(fullPath),
				gitID:         entry.getID(),
				alreadyExists: true,
			}
		} else {
			// gitID is not for a tree
			entryObj = &entryBlob{
				name:  path.Base(fullPath),
				gitID: entry.getID(),
			}
		}
	} else {
		// => This is an intermediate node, has to be a tree that we must build
		entryObj = &entryTree{
			name:          path.Base(fullPath),
			gitID:         ZeroHash,
			alreadyExists: false,
		}
		t.trees[fullPath] = &entryTree{}
	}

	t.trees[parent].entries = append(t.trees[parent].entries, entryObj)
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
func (t *TreeBuilder) writeTree(entries []TreeEntry) (Hash, error) {
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

// TreeEntry represents an entry in a Git tree.
type TreeEntry interface {
	getName() string
	getID() Hash
}

// entryTree implements TreeEntry and indicates the entry is for a Git tree.
type entryTree struct {
	name          string
	gitID         Hash
	alreadyExists bool
	entries       []TreeEntry
}

func (e *entryTree) getName() string {
	return e.name
}

func (e *entryTree) getID() Hash {
	return e.gitID
}

// NewEntryTree creates a TreeEntry that represents a Git tree. If the tree
// doesn't exist, i.e., it must be created, gitID must be set to ZeroHash. The
// name must be set to the full path of the tree object.
func NewEntryTree(name string, gitID Hash) TreeEntry {
	entry := &entryTree{name: name, gitID: gitID}
	if gitID == nil || !gitID.IsZero() {
		entry.alreadyExists = true
	}
	return entry
}

// entryBlob implements TreeEntry and indicates the entry is for a Git blob.
type entryBlob struct {
	name        string
	gitID       Hash
	permissions os.FileMode //nolint:unused
}

func (e *entryBlob) getName() string {
	return e.name
}

func (e *entryBlob) getID() Hash {
	return e.gitID
}

// NewEntryBlob creates a TreeEntry that represents a Git blob.
func NewEntryBlob(name string, gitID Hash) TreeEntry {
	return &entryBlob{name: name, gitID: gitID, permissions: 0o644}
}

// NewEntryBlobWithPermissions creates a TreeEntry that represents a Git blob.
// The permissions parameter can be used to set custom permissions.
func NewEntryBlobWithPermissions(name string, gitID Hash, permissions os.FileMode) TreeEntry {
	return &entryBlob{name: name, gitID: gitID, permissions: permissions}
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
