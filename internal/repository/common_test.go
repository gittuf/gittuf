// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"testing"

	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/stretchr/testify/assert"
)

func assertLocalAndRemoteRefsMatch(t *testing.T, localRepo, remoteRepo *gitinterface.Repository, refName string) {
	t.Helper()

	localRefTip, err := localRepo.GetReference(refName)
	if err != nil {
		t.Fatal(err)
	}

	remoteRefTip, err := remoteRepo.GetReference(refName)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, localRefTip, remoteRefTip)
}
