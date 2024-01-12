// SPDX-License-Identifier: Apache-2.0

package common

import (
	"context"
	"fmt"
	"testing"

	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/stretchr/testify/assert"
)

func TestLoadSigner(t *testing.T) {
	tests := map[string]struct {
		keyBytes []byte
	}{
		"SSH RSA key":   {keyBytes: artifacts.SSHRSAPrivate},
		"SSH ECDSA key": {keyBytes: artifacts.SSHECDSAPrivate},
		"Legacy key":    {keyBytes: artifacts.SSLibKey1Private},
	}

	for name, test := range tests {
		signer, err := LoadSigner(test.keyBytes)
		assert.Nil(t, err, fmt.Sprintf("unexpected error in test '%s'", name))

		_, err = signer.Sign(context.Background(), nil)
		assert.Nil(t, err)
	}
}
