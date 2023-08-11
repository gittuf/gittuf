package tuf

// This package defines gittuf's take on TUF metadata. There are some minor
// changes, such as the addition of `custom` to delegation entries. Some of it,
// however, is inspired by or cloned from the go-tuf implementation.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"path"
	"strings"

	"github.com/secure-systems-lab/go-securesystemslib/cjson"
	"github.com/secure-systems-lab/go-securesystemslib/signerverifier"
)

const (
	GitPatternPrefix  = "git:"
	FilePatternPrefix = "file:"
	specVersion       = "1.0"
)

var (
	ErrTargetsNotEmpty          = errors.New("`targets` field in gittuf Targets metadata must be empty")
	ErrInvalidDelegationPattern = errors.New("invalid pattern, must contain at most one git pattern and one file pattern")
)

// Key defines the structure for how public keys are stored in TUF metadata.
type Key = signerverifier.SSLibKey

// LoadKeyFromBytes returns a pointer to a Key instance created from the
// contents of the bytes. The key contents are expected to be in the custom
// securesystemslib format.
func LoadKeyFromBytes(contents []byte) (*Key, error) {
	// FIXME: this assumes keys are stored in securesystemslib format.
	// RSA keys are stored in PEM format.
	var key *Key
	if err := json.Unmarshal(contents, &key); err != nil {
		return nil, err
	}

	if len(key.KeyID) == 0 {
		keyID, err := calculateKeyID(key)
		if err != nil {
			return nil, err
		}
		key.KeyID = keyID
	}

	return key, nil
}

func calculateKeyID(k *Key) (string, error) {
	key := map[string]any{
		"keytype":               k.KeyType,
		"scheme":                k.Scheme,
		"keyid_hash_algorithms": k.KeyIDHashAlgorithms,
		"keyval": map[string]string{
			"public": k.KeyVal.Public,
		},
	}
	canonical, err := cjson.EncodeCanonical(key)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(canonical)
	return hex.EncodeToString(digest[:]), nil
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
	Keys               map[string]*Key `json:"keys"`
	Roles              map[string]Role `json:"roles"`
}

// NewRootMetadata returns a new instance of RootMetadata.
func NewRootMetadata() *RootMetadata {
	return &RootMetadata{
		Type:               "root",
		SpecVersion:        specVersion,
		ConsistentSnapshot: true,
	}
}

// SetVersion sets the version of the RootMetadata to the value passed in.
func (r *RootMetadata) SetVersion(version int) {
	r.Version = version
}

// SetExpires sets the expiry date of the RootMetadata to the value passed in.
func (r *RootMetadata) SetExpires(expires string) {
	r.Expires = expires
}

// AddKey adds a key to the RootMetadata instance.
func (r *RootMetadata) AddKey(key *Key) {
	if r.Keys == nil {
		r.Keys = map[string]*Key{}
	}

	if key.KeyVal.Private != "" {
		key.KeyVal.Private = ""
	}

	r.Keys[key.KeyID] = key
}

// AddRole adds a role object and associates it with roleName in the
// RootMetadata instance.
func (r *RootMetadata) AddRole(roleName string, role Role) {
	if r.Roles == nil {
		r.Roles = map[string]Role{}
	}

	r.Roles[roleName] = role
}

// TargetsMetadata defines the schema of TUF's Targets role.
type TargetsMetadata struct {
	Type        string         `json:"type"`
	SpecVersion string         `json:"spec_version"`
	Version     int            `json:"version"`
	Expires     string         `json:"expires"`
	Targets     map[string]any `json:"targets"`
	Delegations *Delegations   `json:"delegations"`
}

// NewTargetsMetadata returns a new instance of TargetsMetadata.
func NewTargetsMetadata() *TargetsMetadata {
	return &TargetsMetadata{
		Type:        "targets",
		SpecVersion: specVersion,
		Delegations: &Delegations{},
	}
}

// SetVersion sets the version of the TargetsMetadata to the value passed in.
func (t *TargetsMetadata) SetVersion(version int) {
	t.Version = version
}

// SetExpires sets the expiry date of the TargetsMetadata to the value passed
// in.
func (t *TargetsMetadata) SetExpires(expires string) {
	t.Expires = expires
}

// Validate ensures the instance of TargetsMetadata matches gittuf expectations.
func (t *TargetsMetadata) Validate() error {
	if len(t.Targets) != 0 {
		return ErrTargetsNotEmpty
	}
	return nil
}

// Delegations defines the schema for specifying delegations in TUF's Targets
// metadata.
type Delegations struct {
	Keys  map[string]*Key `json:"keys"`
	Roles []Delegation    `json:"roles"`
}

// AddKey adds a delegations key.
func (d *Delegations) AddKey(key *Key) {
	if d.Keys == nil {
		d.Keys = map[string]*Key{}
	}

	if key.KeyVal.Private != "" {
		key.KeyVal.Private = ""
	}

	d.Keys[key.KeyID] = key
}

// AddDelegation adds a new delegation.
func (d *Delegations) AddDelegation(delegation Delegation) {
	if d.Roles == nil {
		d.Roles = []Delegation{}
	}

	d.Roles = append(d.Roles, delegation)
}

// SortedDelegations sorts and returns all delegations in the Targets metadata.
// The sorting ensures that delegations that apply policies against both
// namespaces are prioritized.
func (d *Delegations) SortedDelegations() []Delegation {
	sortedDelegations := make([]Delegation, 0, len(d.Roles))

	delegationsOverBoth := make([]Delegation, 0, len(d.Roles))
	delegationsOverOne := make([]Delegation, 0, len(d.Roles))

	for _, delegation := range d.Roles {
		if delegation.IsBothNamespaces() {
			delegationsOverBoth = append(delegationsOverBoth, delegation)
		} else {
			delegationsOverOne = append(delegationsOverOne, delegation)
		}
	}

	sortedDelegations = append(sortedDelegations, delegationsOverBoth...)
	sortedDelegations = append(sortedDelegations, delegationsOverOne...)
	return sortedDelegations
}

// Delegation defines the schema for a single delegation entry. It differs from
// the standard TUF schema by allowing a `custom` field to record details
// pertaining to the delegation.
type Delegation struct {
	Name        string            `json:"name"`
	Paths       []*DelegationPath `json:"paths"`
	Terminating bool              `json:"terminating"`
	Custom      *json.RawMessage  `json:"custom,omitempty"`
	Role
}

// Matches checks if any of the delegation's patterns match the target.
func (d *Delegation) Matches(gitRef string, file string) bool {
	for _, rule := range d.Paths {
		if ok := rule.Matches(gitRef, file); ok {
			return true
		}
	}
	return false
}

func (d *Delegation) IsBothNamespaces() bool {
	for _, rule := range d.Paths {
		gitRule, fileRule := rule.RuleType()
		if gitRule && fileRule {
			return true
		}
	}

	return false
}

type DelegationPath struct {
	GitRefPattern string `json:"git_ref_pattern"`
	FilePattern   string `json:"file_pattern"`
}

func NewDelegationPath(pattern string) (*DelegationPath, error) {
	before, after, found := strings.Cut(pattern, "&&")
	var g, f string = "*", "*"
	if found {
		// Identify which of before and after is which type
		if strings.HasPrefix(before, GitPatternPrefix) && strings.HasPrefix(after, FilePatternPrefix) {
			g = strings.TrimPrefix(before, GitPatternPrefix)
			f = strings.TrimPrefix(after, FilePatternPrefix)
		} else if strings.HasPrefix(before, FilePatternPrefix) && strings.HasPrefix(after, GitPatternPrefix) {
			g = strings.TrimPrefix(after, GitPatternPrefix)
			f = strings.TrimPrefix(before, FilePatternPrefix)
		} else {
			return nil, ErrInvalidDelegationPattern
		}
	} else {
		if strings.HasPrefix(before, GitPatternPrefix) {
			g = strings.TrimPrefix(before, GitPatternPrefix)
		} else if strings.HasPrefix(before, FilePatternPrefix) {
			f = strings.TrimPrefix(before, FilePatternPrefix)
		} else {
			return nil, ErrInvalidDelegationPattern
		}
	}

	return &DelegationPath{GitRefPattern: g, FilePattern: f}, nil
}

func (d *DelegationPath) Matches(gitRef, file string) bool {
	gitRule, fileRule := d.RuleType()

	gitMatched, _ := path.Match(d.GitRefPattern, gitRef)
	fileMatched, _ := path.Match(d.FilePattern, file)

	if gitRule && fileRule {
		return gitMatched && fileMatched
	} else if gitRule {
		return gitMatched
	}

	return fileMatched
}

func (d *DelegationPath) RuleType() (bool, bool) {
	return d.GitRefPattern != "*", d.FilePattern != "*"
}
