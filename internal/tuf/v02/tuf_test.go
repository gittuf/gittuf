// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package v02

import (
	"fmt"
	"testing"

	"github.com/gittuf/gittuf/internal/signerverifier/ssh"
	"github.com/secure-systems-lab/go-securesystemslib/signerverifier"
	"github.com/stretchr/testify/assert"
)

func TestPerson(t *testing.T) {
	keyR := ssh.NewKeyFromBytes(t, rootPubKeyBytes)
	key := NewKeyFromSSLibKey(keyR)

	tests := map[string]struct {
		person                 *Person
		expectedID             string
		expectedKeys           []*signerverifier.SSLibKey
		expectedCustomMetadata map[string]string
	}{
		"no custom metadata": {
			person: &Person{
				PersonID: "jane.doe",
				PublicKeys: map[string]*Key{
					key.KeyID: key,
				},
			},
			expectedID:             "jane.doe",
			expectedKeys:           []*signerverifier.SSLibKey{keyR},
			expectedCustomMetadata: nil,
		},
		"only associated identities": {
			person: &Person{
				PersonID: "jane.doe",
				PublicKeys: map[string]*Key{
					key.KeyID: key,
				},
				AssociatedIdentities: map[string]string{
					"https://github.com": "jane.doe",
					"https://gitlab.com": "jane.doe",
				},
			},
			expectedID:   "jane.doe",
			expectedKeys: []*signerverifier.SSLibKey{keyR},
			expectedCustomMetadata: map[string]string{
				fmt.Sprintf("%s https://github.com", associatedIdentityKey): "jane.doe",
				fmt.Sprintf("%s https://gitlab.com", associatedIdentityKey): "jane.doe",
			},
		},
		"only custom metadata": {
			person: &Person{
				PersonID: "jane.doe",
				PublicKeys: map[string]*Key{
					key.KeyID: key,
				},
				Custom: map[string]string{
					"key": "value",
				},
			},
			expectedID:   "jane.doe",
			expectedKeys: []*signerverifier.SSLibKey{keyR},
			expectedCustomMetadata: map[string]string{
				"key": "value",
			},
		},
		"both associated identities and custom metadata": {
			person: &Person{
				PersonID: "jane.doe",
				PublicKeys: map[string]*Key{
					key.KeyID: key,
				},
				AssociatedIdentities: map[string]string{
					"https://github.com": "jane.doe",
					"https://gitlab.com": "jane.doe",
				},
				Custom: map[string]string{
					"key": "value",
				},
			},
			expectedID:   "jane.doe",
			expectedKeys: []*signerverifier.SSLibKey{keyR},
			expectedCustomMetadata: map[string]string{
				fmt.Sprintf("%s https://github.com", associatedIdentityKey): "jane.doe",
				fmt.Sprintf("%s https://gitlab.com", associatedIdentityKey): "jane.doe",
				"key": "value",
			},
		},
	}

	for name, test := range tests {
		id := test.person.ID()
		assert.Equal(t, test.expectedID, id, fmt.Sprintf("unexpected person ID in test '%s'", name))

		keys := test.person.Keys()
		assert.Equal(t, test.expectedKeys, keys, fmt.Sprintf("unexpected keys in test '%s'", name))

		customMetadata := test.person.CustomMetadata()
		assert.Equal(t, test.expectedCustomMetadata, customMetadata, fmt.Sprintf("unexpected custom metadata in test '%s'", name))
	}
}
