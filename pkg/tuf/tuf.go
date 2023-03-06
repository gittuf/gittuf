package tuf

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

type Key struct {
	KeyType string `json:"keytype"`
	Scheme  string `json:"scheme"`
	KeyVal  KeyVal `json:"keyval"`
	keyID   string
	idOnce  sync.Once
}

type KeyVal struct {
	Public  string `json:"public"`
	private string
}

func LoadKeyFromBytes(contents []byte) (Key, error) {
	// FIXME: this assumes keys are stored in securesystemslib format.
	var key Key
	if err := json.Unmarshal(contents, &key); err != nil {
		return Key{}, err
	}
	return key, nil
}

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

type Role struct {
	KeyIDs    []string `json:"keyids"`
	Threshold int      `json:"threshold"`
}

type RootMetadata struct {
	Type               string          `json:"type"`
	SpecVersion        string          `json:"spec_version"`
	ConsistentSnapshot bool            `json:"consistent_snapshot"` // TODO: how do we handle this?
	Version            int             `json:"version"`
	Expires            string          `json:"expires"`
	Keys               map[string]Key  `json:"keys"`
	Roles              map[string]Role `json:"roles"`
}

type TargetsMetadata struct {
	Type        string                 `json:"type"`
	SpecVersion string                 `json:"spec_version"`
	Version     int                    `json:"version"`
	Expires     string                 `json:"expires"`
	Targets     map[string]interface{} `json:"targets"`
	Delegations *Delegations           `json:"delegations"`
}

type Delegations struct {
	Keys  map[string]Key `json:"keys"`
	Roles []*Delegation  `json:"roles"`
}

type Delegation struct {
	Name        string           `json:"name"`
	Paths       []string         `json:"paths"`
	Terminating bool             `json:"terminating"`
	Custom      *json.RawMessage `json:"custom,omitempty"`
	Role
}

func (t *TargetsMetadata) Validate() error {
	if len(t.Targets) != 0 {
		return ErrTargetsNotEmpty
	}
	return nil
}
