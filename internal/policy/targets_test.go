// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"testing"

	tufv02 "github.com/gittuf/gittuf/internal/tuf/v02"
	"github.com/stretchr/testify/assert"
)

func TestInitializeTargetsMetadata(t *testing.T) {
	targetsMetadata := InitializeTargetsMetadata()

	assert.Contains(t, targetsMetadata.GetRules(), tufv02.AllowRule())
}
