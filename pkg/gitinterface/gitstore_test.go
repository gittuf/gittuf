// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"testing"

	"github.com/gittuf/gittuf/pkg/gitstore"
	"github.com/stretchr/testify/assert"
)

var _ gitstore.Storer = (*Repository)(nil)

func TestErrReferenceNotFoundIsGitstoreSentinel(t *testing.T) {
	t.Parallel()
	assert.ErrorIs(t, ErrReferenceNotFound, gitstore.ErrReferenceNotFound)
}
