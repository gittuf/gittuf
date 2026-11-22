// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitstore

import (
	"github.com/gittuf/gittuf/pkg/githash"
)

// CommitInfo is the subset of a commit's contents read by GetCommitInfo.
// ParentIDs lists all parents in order and is nil when the commit has none or
// is a shallow boundary. Message is trimmed of leading and trailing whitespace.
type CommitInfo struct {
	TreeID    githash.Hash
	ParentIDs []githash.Hash
	Message   string
}
