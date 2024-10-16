// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"testing"

	"github.com/gittuf/gittuf/internal/signerverifier/ssh"
	"github.com/gittuf/gittuf/internal/tuf"
	tufv01 "github.com/gittuf/gittuf/internal/tuf/v01"
	"github.com/stretchr/testify/assert"
)

func TestInitializeRootMetadata(t *testing.T) {
	key := tufv01.NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, rootPubKeyBytes))

	rootMetadata, err := InitializeRootMetadata(key)
	assert.Nil(t, err)

	allPrincipals := rootMetadata.GetPrincipals()
	assert.Equal(t, key, allPrincipals[key.KeyID])

	threshold, err := rootMetadata.GetRootThreshold()
	assert.Nil(t, err)
	assert.Equal(t, 1, threshold)

	rootPrincipals, err := rootMetadata.GetRootPrincipals()
	assert.Nil(t, err)
	assert.Equal(t, []tuf.Principal{key}, rootPrincipals)
}
