// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"bytes"
	"fmt"
	"path"
	"strings"
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

// TreeBuilder is used to create multi-level trees in a repository.  Based on
// `buildTreeHelper` in go-git.
type TreeBuilder struct {
	repo    *Repository
	trees   map[string]*entry
	entries map[string]*entry
}

func NewTreeBuilder(repo *Repository) *TreeBuilder {
	return &TreeBuilder{repo: repo}
}

// WriteRootTreeFromBlobIDs accepts a map of paths to their blob IDs and returns
// the root tree ID that contains these files.
func (t *TreeBuilder) WriteRootTreeFromBlobIDs(files map[string]Hash) (Hash, error) {
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
func (t *TreeBuilder) buildIntermediates(name string, gitID Hash) {
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
func (t *TreeBuilder) buildTree(name, parent, fullPath string, gitID Hash) {
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
func (t *TreeBuilder) writeTrees(parent string, tree *entry) (Hash, error) {
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
func (t *TreeBuilder) writeTree(entries []*entry) (Hash, error) {
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

// entry is a helper type that represents an entry in a Git tree. If `isDir` is
// true, it indicates the entry represents a subtree.
type entry struct {
	name    string
	isDir   bool
	gitID   Hash
	entries []*entry // only used when isDir is true
}
