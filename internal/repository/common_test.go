// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
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
