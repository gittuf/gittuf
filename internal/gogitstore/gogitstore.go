// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

// Package gogitstore provides experimental in-process Git reads.
package gogitstore

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/gittuf/gittuf/pkg/githash"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/gittuf/gittuf/pkg/gitstore"
	gogit "github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/filemode"
	"github.com/go-git/go-git/v6/plumbing/object"
	"github.com/go-git/go-git/v6/storage/memory"
)

// ErrInvalidHash is returned when a hash is not a valid go-git object ID.
var ErrInvalidHash = errors.New("hash is not a valid Git object ID")

// Git resolves the empty tree even when it is absent from the object database.
const (
	emptyTreeSHA1   = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
	emptyTreeSHA256 = "6ef19b41225c5369f1c104d45d8d85efa9b057b53b14b4b9b939dd74decc5321"
)

var _ gitstore.Storer = (*Storer)(nil)

// Storer caches a go-git handle and delegates other operations to Git.
type Storer struct {
	*gitinterface.Repository

	mu      sync.Mutex
	gogit   *gogit.Repository
	enabled bool

	trace *Trace
}

// New wraps repo. With enableGoGit false every method delegates, leaving
// behaviour identical to gitinterface.
func New(repo *gitinterface.Repository, enableGoGit bool) *Storer {
	return &Storer{Repository: repo, enabled: enableGoGit}
}

// NewWithTrace is New with tracing attached.
func NewWithTrace(repo *gitinterface.Repository, enableGoGit bool, trace *Trace) *Storer {
	return &Storer{Repository: repo, enabled: enableGoGit, trace: trace}
}

// record logs one call. Callers defer it, so start is evaluated on entry.
func (s *Storer) record(method string, start time.Time, delegated bool) {
	if s.trace == nil {
		return
	}
	s.trace.record(method, start, delegated)
}

// fast returns the cached go-git handle, opening it if needed. A nil handle
// and nil error mean the backend is off and the caller should delegate.
func (s *Storer) fast() (*gogit.Repository, error) {
	if !s.enabled {
		return nil, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.gogit != nil {
		return s.gogit, nil
	}

	repo, err := s.GetGoGitRepository()
	if err != nil {
		return nil, err
	}
	s.gogit = repo

	slog.Debug("Opened in-process go-git repository handle")
	if s.trace != nil {
		s.trace.recordHandleOpen()
	}

	return repo, nil
}

// invalidate drops the cached handle. go-git looks objects up in a packfile
// index it caches on open, so it would miss a packfile written afterwards.
func (s *Storer) invalidate() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gogit = nil
}

func (s *Storer) ReadBlob(blobID githash.Hash) ([]byte, error) {
	repo, err := s.fast()
	if err != nil {
		return nil, err
	}
	defer s.record("ReadBlob", time.Now(), repo == nil)

	if repo == nil {
		return s.Repository.ReadBlob(blobID)
	}

	id, err := toGoGit(blobID)
	if err != nil {
		return nil, err
	}

	// BlobObject already fails on a non-blob, so no separate type check.
	blob, err := repo.BlobObject(id)
	if err != nil {
		return nil, fmt.Errorf("unable to read blob '%s': %w", blobID.String(), err)
	}

	reader, err := blob.Reader()
	if err != nil {
		return nil, fmt.Errorf("unable to read blob '%s': %w", blobID.String(), err)
	}
	defer reader.Close() //nolint:errcheck

	return io.ReadAll(reader)
}

func (s *Storer) GetCommitMessage(commitID githash.Hash) (string, error) {
	repo, err := s.fast()
	if err != nil {
		return "", err
	}
	defer s.record("GetCommitMessage", time.Now(), repo == nil)

	if repo == nil {
		return s.Repository.GetCommitMessage(commitID)
	}

	commit, err := s.commit(repo, commitID)
	if err != nil {
		return "", fmt.Errorf("unable to identify message for commit '%s': %w", commitID.String(), err)
	}

	// Match gitinterface whitespace trimming.
	return strings.TrimSpace(commit.Message), nil
}

func (s *Storer) GetCommitTreeID(commitID githash.Hash) (githash.Hash, error) {
	repo, err := s.fast()
	if err != nil {
		return nil, err
	}
	defer s.record("GetCommitTreeID", time.Now(), repo == nil)

	if repo == nil {
		return s.Repository.GetCommitTreeID(commitID)
	}

	commit, err := s.commit(repo, commitID)
	if err != nil {
		return s.ZeroHash(), fmt.Errorf("unable to identify tree for commit '%s': %w", commitID.String(), err)
	}

	return fromGoGit(commit.TreeHash)
}

// GetCommitParentIDs returns nil for a commit with no parents, as gitinterface
// does.
func (s *Storer) GetCommitParentIDs(commitID githash.Hash) ([]githash.Hash, error) {
	repo, err := s.fast()
	if err != nil {
		return nil, err
	}
	defer s.record("GetCommitParentIDs", time.Now(), repo == nil)

	if repo == nil {
		return s.Repository.GetCommitParentIDs(commitID)
	}

	commit, err := s.commit(repo, commitID)
	if err != nil {
		return nil, fmt.Errorf("unable to identify parents for commit '%s': %w", commitID.String(), err)
	}

	shallow, err := repo.Storer.Shallow()
	if err != nil {
		return nil, err
	}
	for _, boundary := range shallow {
		if boundary == commit.Hash {
			return nil, nil
		}
	}

	if len(commit.ParentHashes) == 0 {
		return nil, nil
	}

	parentIDs := make([]githash.Hash, 0, len(commit.ParentHashes))
	for _, parentHash := range commit.ParentHashes {
		parentID, err := fromGoGit(parentHash)
		if err != nil {
			return nil, fmt.Errorf("invalid parent commit ID '%s': %w", parentHash.String(), err)
		}
		parentIDs = append(parentIDs, parentID)
	}

	return parentIDs, nil
}

// GetEntriesInTree returns nil for an empty tree, as gitinterface does.
func (s *Storer) GetEntriesInTree(treeID githash.Hash) ([]gitstore.TreeEntry, error) {
	repo, err := s.fast()
	if err != nil {
		return nil, err
	}
	defer s.record("GetEntriesInTree", time.Now(), repo == nil)

	if repo == nil {
		return s.Repository.GetEntriesInTree(treeID)
	}

	tree, err := s.tree(repo, treeID)
	if err != nil {
		return nil, fmt.Errorf("unable to enumerate items in tree '%s': %w", treeID.String(), err)
	}

	if len(tree.Entries) == 0 {
		return nil, nil
	}

	entries := make([]gitstore.TreeEntry, 0, len(tree.Entries))
	for _, entry := range tree.Entries {
		id, err := fromGoGit(entry.Hash)
		if err != nil {
			return nil, fmt.Errorf("invalid Git ID '%s' for path '%s': %w", entry.Hash.String(), entry.Name, err)
		}

		// ls-tree only reports trees as "tree". Everything else, including
		// submodule gitlinks, counts as a blob.
		kind := gitstore.KindBlob
		if entry.Mode == filemode.Dir {
			kind = gitstore.KindSubtree
		}

		entries = append(entries, gitstore.TreeEntry{Path: entry.Name, ID: id, Kind: kind})
	}

	return entries, nil
}

// GetAllFilesInTree returns nil for an empty tree, as gitinterface does.
func (s *Storer) GetAllFilesInTree(treeID githash.Hash) (map[string]githash.Hash, error) {
	repo, err := s.fast()
	if err != nil {
		return nil, err
	}
	defer s.record("GetAllFilesInTree", time.Now(), repo == nil)

	if repo == nil {
		return s.Repository.GetAllFilesInTree(treeID)
	}

	files := map[string]githash.Hash{}
	if err := s.collectFiles(repo, treeID, "", files); err != nil {
		return nil, fmt.Errorf("unable to enumerate all files in tree: %w", err)
	}

	if len(files) == 0 {
		return nil, nil
	}

	return files, nil
}

// collectFiles records every non-tree entry. Submodule gitlinks are leaves,
// matching ls-tree -r.
func (s *Storer) collectFiles(repo *gogit.Repository, treeID githash.Hash, prefix string, files map[string]githash.Hash) error {
	tree, err := s.tree(repo, treeID)
	if err != nil {
		return err
	}

	for _, entry := range tree.Entries {
		path := entry.Name
		if prefix != "" {
			path = prefix + "/" + entry.Name
		}

		id, err := fromGoGit(entry.Hash)
		if err != nil {
			return fmt.Errorf("invalid Git ID '%s' for path '%s': %w", entry.Hash.String(), path, err)
		}

		if entry.Mode == filemode.Dir {
			if err := s.collectFiles(repo, id, path, files); err != nil {
				return err
			}
			continue
		}

		files[path] = id
	}

	return nil
}

// GetPathIDInTree resolves a slash separated path. An intermediate path
// returns that subtree's ID.
func (s *Storer) GetPathIDInTree(treeID githash.Hash, treePath string) (githash.Hash, error) {
	repo, err := s.fast()
	if err != nil {
		return nil, err
	}
	defer s.record("GetPathIDInTree", time.Now(), repo == nil)

	if repo == nil {
		return s.Repository.GetPathIDInTree(treeID, treePath)
	}

	components := strings.Split(strings.TrimSuffix(treePath, "/"), "/")

	currentTreeID := treeID
	for len(components) != 0 {
		entries, err := s.GetEntriesInTree(currentTreeID)
		if err != nil {
			return nil, err
		}

		entryID, has := findEntryID(entries, components[0])
		if !has {
			return nil, fmt.Errorf("%w: %s", gitinterface.ErrTreeDoesNotHavePath, treePath)
		}

		currentTreeID = entryID
		components = components[1:]
	}

	return currentTreeID, nil
}

func findEntryID(entries []gitstore.TreeEntry, name string) (githash.Hash, bool) {
	for _, entry := range entries {
		if entry.Path == name {
			return entry.ID, true
		}
	}
	return nil, false
}

// GetReference delegates anything that is not a full reference name.
// gitinterface uses rev-parse, which also accepts revision expressions such as
// short branch names and object IDs.
func (s *Storer) GetReference(refName string) (githash.Hash, error) {
	repo, err := s.fast()
	if err != nil {
		return nil, err
	}
	delegated := repo == nil || !isFullReferenceName(refName)
	defer s.record("GetReference", time.Now(), delegated)

	if delegated {
		return s.Repository.GetReference(refName)
	}

	reference, err := repo.Reference(plumbing.ReferenceName(refName), true)
	if err != nil {
		if errors.Is(err, plumbing.ErrReferenceNotFound) {
			return s.ZeroHash(), gitstore.ErrReferenceNotFound
		}
		return s.ZeroHash(), fmt.Errorf("unable to read reference '%s': %w", refName, err)
	}

	return fromGoGit(reference.Hash())
}

func isFullReferenceName(refName string) bool {
	return refName == "HEAD" || (strings.HasPrefix(refName, "refs/") && plumbing.ReferenceName(refName).Validate() == nil)
}

// GetTagTarget peels chained tags down to a commit. gitinterface uses
// rev-list -n 1, which does the same, not the tag's immediate target.
func (s *Storer) GetTagTarget(tagID githash.Hash) (githash.Hash, error) {
	repo, err := s.fast()
	if err != nil {
		return nil, err
	}
	defer s.record("GetTagTarget", time.Now(), repo == nil)

	if repo == nil {
		return s.Repository.GetTagTarget(tagID)
	}

	id, err := toGoGit(tagID)
	if err != nil {
		return s.ZeroHash(), err
	}

	for {
		tag, err := repo.TagObject(id)
		if err != nil {
			break
		}
		id = tag.Target
	}

	commit, err := repo.CommitObject(id)
	if err != nil {
		return s.ZeroHash(), fmt.Errorf("unable to resolve tag's target ID: %w", err)
	}

	return fromGoGit(commit.Hash)
}

// GetObjectSignature is already go-git in gitinterface. Overriding it reuses
// the cached handle and drops the two cat-file calls used to find the type.
func (s *Storer) GetObjectSignature(objectID githash.Hash) ([]byte, []byte, error) {
	repo, err := s.fast()
	if err != nil {
		return nil, nil, err
	}
	defer s.record("GetObjectSignature", time.Now(), repo == nil)

	if repo == nil {
		return s.Repository.GetObjectSignature(objectID)
	}

	id, err := toGoGit(objectID)
	if err != nil {
		return nil, nil, err
	}

	obj, err := repo.Object(plumbing.AnyObject, id)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to load object '%s': %w", objectID.String(), err)
	}

	switch obj := obj.(type) {
	case *object.Commit:
		payload, err := encodeWithoutSignature(obj)
		if err != nil {
			return nil, nil, fmt.Errorf("unable to encode commit contents: %w", err)
		}

		// SHA-256 commits sign under gpgsig-sha256, which go-git keeps apart
		// from the SHA-1 gpgsig header.
		signature := obj.Signature
		if objectID.IsSHA256() {
			signature = obj.SignatureSHA256
		}

		return payload, []byte(signature), nil

	case *object.Tag:
		payload, err := encodeWithoutSignature(obj)
		if err != nil {
			return nil, nil, fmt.Errorf("unable to encode tag contents: %w", err)
		}

		// Tag signatures are appended to the payload in both object formats.
		return payload, []byte(obj.Signature), nil
	}

	return nil, nil, gitinterface.ErrNotCommitOrTag
}

// signatureless is implemented by go-git commits and tags, which can re-encode
// themselves as the bytes their signature covered.
type signatureless interface {
	EncodeWithoutSignature(o plumbing.EncodedObject) error
}

func encodeWithoutSignature(obj signatureless) ([]byte, error) {
	encoded := memory.NewStorage().NewEncodedObject()
	if err := obj.EncodeWithoutSignature(encoded); err != nil {
		return nil, err
	}

	reader, err := encoded.Reader()
	if err != nil {
		return nil, err
	}
	defer reader.Close() //nolint:errcheck

	return io.ReadAll(reader)
}

func (s *Storer) commit(repo *gogit.Repository, commitID githash.Hash) (*object.Commit, error) {
	id, err := toGoGit(commitID)
	if err != nil {
		return nil, err
	}
	return repo.CommitObject(id)
}

func (s *Storer) tree(repo *gogit.Repository, treeID githash.Hash) (*object.Tree, error) {
	if isEmptyTree(treeID) {
		return &object.Tree{}, nil
	}

	id, err := toGoGit(treeID)
	if err != nil {
		return nil, err
	}
	return repo.TreeObject(id)
}

func isEmptyTree(treeID githash.Hash) bool {
	id := treeID.String()
	return id == emptyTreeSHA1 || id == emptyTreeSHA256
}

func toGoGit(h githash.Hash) (plumbing.Hash, error) {
	id, ok := plumbing.FromHex(h.String())
	if !ok {
		return plumbing.ZeroHash, fmt.Errorf("%w: %s", ErrInvalidHash, h.String())
	}
	return id, nil
}

func fromGoGit(id plumbing.Hash) (githash.Hash, error) {
	return githash.NewHash(id.String())
}
