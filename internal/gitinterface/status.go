// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"errors"
	"fmt"
	"strings"
)

// See https://git-scm.com/docs/git-status#_porcelain_format_version_1.

var (
	ErrInvalidStatusCodeLength = errors.New("status code string must be of length 1")
	ErrInvalidStatusCode       = errors.New("status code string is unrecognized")
)

type StatusCode uint

const (
	StatusCodeUnmodified StatusCode = iota + 1 // we use 0 as error code
	StatusCodeModified
	StatusCodeTypeChanged
	StatusCodeAdded
	StatusCodeDeleted
	StatusCodeRenamed
	StatusCodeCopied
	StatusCodeUpdatedUnmerged
	StatusCodeUntracked
	StatusCodeIgnored
)

func (s StatusCode) String() string {
	switch s {
	case StatusCodeUnmodified:
		return " " // is this actually a space or empty string?
	case StatusCodeModified:
		return "M"
	case StatusCodeTypeChanged:
		return "T"
	case StatusCodeAdded:
		return "A"
	case StatusCodeDeleted:
		return "D"
	case StatusCodeRenamed:
		return "R"
	case StatusCodeCopied:
		return "C"
	case StatusCodeUpdatedUnmerged:
		return "U"
	case StatusCodeUntracked:
		return "?"
	case StatusCodeIgnored:
		return "!"
	default:
		return "invalid-code"
	}
}

func NewStatusCodeFromByte(s byte) (StatusCode, error) {
	switch s {
	case ' ':
		return StatusCodeUnmodified, nil
	case 'M':
		return StatusCodeModified, nil
	case 'T':
		return StatusCodeTypeChanged, nil
	case 'A':
		return StatusCodeAdded, nil
	case 'D':
		return StatusCodeDeleted, nil
	case 'C':
		return StatusCodeCopied, nil
	case 'U':
		return StatusCodeUpdatedUnmerged, nil
	case '?':
		return StatusCodeUntracked, nil
	case '!':
		return StatusCodeIgnored, nil
	default:
		return 0, ErrInvalidStatusCode
	}
}

type FileStatus struct {
	X StatusCode
	Y StatusCode
}

func (f *FileStatus) Untracked() bool {
	return f.X == StatusCodeUntracked || f.Y == StatusCodeUntracked
}

func (r *Repository) Status() (map[string]FileStatus, error) {
	worktree := r.gitDirPath
	if !r.IsBare() {
		worktree = strings.TrimSuffix(worktree, ".git") // TODO: this doesn't support detached git dir
	}

	output, err := r.executor("-C", worktree, "status", "--porcelain=1", "-z", "--untracked-files=all", "--ignored").executeString()

	if err != nil {
		return nil, fmt.Errorf("unable to check status of repository: %w", err)
	}

	statuses := map[string]FileStatus{}

	lines := strings.Split(output, string('\000'))
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}

		// first two characters are status codes, find the corresponding
		// statuses
		xb := line[0]
		yb := line[1]
		// Note: we identify the status after inspecting the path so we can
		// provide better error messages

		// then, we have a single space followed by the path, ignore space and
		// read in the rest as the filepath
		filePath := strings.TrimSpace(line[2:])

		xStatus, err := NewStatusCodeFromByte(xb)
		if err != nil {
			return nil, fmt.Errorf("unable to parse status code '%c' for path '%s': %w", xb, filePath, err)
		}

		yStatus, err := NewStatusCodeFromByte(yb)
		if err != nil {
			return nil, fmt.Errorf("unable to parse status code '%c' for path '%s': %w", yb, filePath, err)
		}

		status := FileStatus{X: xStatus, Y: yStatus}

		statuses[filePath] = status
	}

	return statuses, nil
}
