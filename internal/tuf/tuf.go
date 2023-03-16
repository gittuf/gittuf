package tuf

// This package defines gittuf's take on TUF metadata. There are some minor
// changes, such as the addition of `custom` to delegation entries. Some of it,
// however, is inspired by or cloned from the go-tuf implementation.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/secure-systems-lab/go-securesystemslib/cjson"
)

var (
	ErrTargetsNotEmpty = errors.New("`targets` field in gittuf Targets metadata must be empty")
)

// Key defines the structure for how public keys are stored in TUF metadata.
type Key struct {
	KeyType             string   `json:"keytype"`
	Scheme              string   `json:"scheme"`
	KeyVal              KeyVal   `json:"keyval"`
	KeyIDHashAlgorithms []string `json:"keyid_hash_algorithms"`
	keyID               string
	idOnce              sync.Once
}

// KeyVal contains a `Public` field that records the public key value.
type KeyVal struct {
	Public string `json:"public"`
}

// LoadKeyFromBytes returns a pointer to a Key instance created from the
// contents of the bytes. The key contents are expected to be in the custom
// securesystemslib format.
func LoadKeyFromBytes(contents []byte) (*Key, error) {
	// FIXME: this assumes keys are stored in securesystemslib format.
	var key Key
	if err := json.Unmarshal(contents, &key); err != nil {
		return nil, err
	}
	return &key, nil
}

// ID returns the key ID.
func (k *Key) ID() string {
	// Modified version of go-tuf's implementation to use a single Key ID.
	k.idOnce.Do(func() {
		data, err := cjson.EncodeCanonical(k)
		if err != nil {
			panic(fmt.Errorf("error creating key ID: %w", err))
		}
		digest := sha256.Sum256(data)
		k.keyID = hex.EncodeToString(digest[:])
	})

	return k.keyID
}

// Role records common characteristics recorded in a role entry in Root metadata
// and in a delegation entry.
type Role struct {
	KeyIDs    []string `json:"keyids"`
	Threshold int      `json:"threshold"`
}

// RootMetadata defines the schema of TUF's Root role.
type RootMetadata struct {
	Type               string          `json:"type"`
	SpecVersion        string          `json:"spec_version"`
	ConsistentSnapshot bool            `json:"consistent_snapshot"` // TODO: how do we handle this?
	Version            int             `json:"version"`
	Expires            string          `json:"expires"`
	Keys               map[string]Key  `json:"keys"`
	Roles              map[string]Role `json:"roles"`
}

// TargetsMetadata defines the schema of TUF's Targets role.
type TargetsMetadata struct {
	Type        string                 `json:"type"`
	SpecVersion string                 `json:"spec_version"`
	Version     int                    `json:"version"`
	Expires     string                 `json:"expires"`
	Targets     map[string]interface{} `json:"targets"`
	Delegations *Delegations           `json:"delegations"`
}

// Delegations defines the schema for specifying delegations in TUF's Targets
// metadata.
type Delegations struct {
	Keys  map[string]Key `json:"keys"`
	Roles []*Delegation  `json:"roles"`
}

// Delegation defines the schema for a single delegation entry. It differs from
// the standard TUF schema by allowing a `custom` field to record details
// pertaining to the delegation.
type Delegation struct {
	Name        string           `json:"name"`
	Paths       []string         `json:"paths"`
	Terminating bool             `json:"terminating"`
	Custom      *json.RawMessage `json:"custom,omitempty"`
	Role
}

// Validate ensures the instance of TargetsMetadata matches gittuf expectations.
func (t *TargetsMetadata) Validate() error {
	if len(t.Targets) != 0 {
		return ErrTargetsNotEmpty
	}
	return nil
}
