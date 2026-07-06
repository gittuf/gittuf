// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package v01

import (
	"testing"

	"github.com/gittuf/gittuf/internal/signerverifier/ssh"
	"github.com/stretchr/testify/assert"
)

func TestKey(t *testing.T) {
	keyR := ssh.NewKeyFromBytes(t, rootPubKeyBytes)
	key := NewKeyFromSSLibKey(keyR)

	metadata := key.CustomMetadata()
	assert.Nil(t, metadata)
}
