// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"testing"

	"github.com/gittuf/gittuf/internal/third_party/go-git"
	"github.com/gittuf/gittuf/internal/third_party/go-git/plumbing"
	"github.com/stretchr/testify/assert"
)

func assertLocalAndRemoteRefsMatch(t *testing.T, localRepo, remoteRepo *git.Repository, refName string) {
	t.Helper()

	refNameT := plumbing.ReferenceName(refName)

	localRef, err := localRepo.Reference(refNameT, true)
	if err != nil {
		t.Fatal(err)
	}

	remoteRef, err := remoteRepo.Reference(refNameT, true)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, localRef.Hash(), remoteRef.Hash())
}
