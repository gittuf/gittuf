// SPDX-License-Identifier: Apache-2.0

package tuf

import (
	"errors"

	"github.com/danwakefield/fnmatch"
)

// The paths.go file contains the definitions and logic for rule paths in
// gittuf. This is in its own file for neatness purposes.

var (
	ErrBranchPathScope = errors.New("branch paths cannot have branch scopes")
)

const (
	INTERNAL_PATH = 0
	BRANCH_PATH   = 1
	FILE_PATH     = 2
)

// Path serves as a container for the different path types we can have in a
// gittuf rule: branch or file paths.
type Path interface {
	PathType() int // 0: reserved, 1: branch, 2: file
	Matches(target []string) bool
}

// // InternalPath is used to store any arbitrary path, preserving the old
// // functionality prior to the addition of BranchPath and FilePath
// type InternalPath struct {
// 	Path string
// }

// // PathType for InternalPath returns 0.
// func (p *InternalPath) PathType() int {
// 	return INTERNAL_PATH
// }

// func (p *InternalPath) GetPath() string {
// 	return p.Path
// }

// BranchPath represents a branch path. There is only one value: the branch it
// represents.
type BranchPath struct {
	BranchName string
}

// PathType for BranchPath returns 1.
func (b *BranchPath) PathType() int {
	return BRANCH_PATH // 0: branch
}

// GetPath for BranchPath returns the branch name.
func (b *BranchPath) GetPath() string {
	return b.BranchName
}

// Matches for BranchPath returns whether the branch pattern matches the input
// target. In this case, the input array is always expected to be of size 1.
func (b *BranchPath) Matches(target []string) bool {
	return fnmatch.Match(b.BranchName, target[0], 0)
}

// FilePath represents a file path. There are two values: the file(s) it
// represents, and any branches that this rule is scoped to.
type FilePath struct {
	FilePath    string
	BranchScope []string
}

// PathType for FilePath returns 2.
func (f *FilePath) PathType() int {
	return FILE_PATH // 1: file
}

// GetPath for FilePath returns the file path.
func (f *FilePath) GetPath() string {
	return f.FilePath
}

// GetBranchScope for FilePath returns the set of branches that this file path
// applies to.
func (f *FilePath) GetBranchScope() []string {
	return f.BranchScope
}

// Matches for FilePath returns whether the file pattern and any scope match the
// input target. The input array may be either of size 1 for only a file path to
// match against, or of size 2, where the second element is the branch that this
// rule applies to.
func (f *FilePath) Matches(target []string) bool {
	// First, check if the file path itself matches the input one
	if matches := fnmatch.Match(f.FilePath, target[0], 0); matches {
		// Check if the path has a scope.

		// Note that if the path does have a scope, then this means that this
		// path only applies in certain branches. If the path does not have one,
		// then it is assumed to apply in all branches.
		if len(f.BranchScope) > 0 {
			// If so, iterate over each branch specified, and check if it maches
			// the one in the input array.
			for _, scope := range f.BranchScope {
				if matches := fnmatch.Match(scope, target[1], 0); matches {
					return true
				}
			}
		} else {
			// If the path does not have any scope information, return true.
			return true
		}
	}
	return false
}
