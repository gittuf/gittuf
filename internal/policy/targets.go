// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"time"

	"github.com/gittuf/gittuf/internal/tuf"
)

const AllowRuleName = "gittuf-allow-rule"

// InitializeTargetsMetadata creates a new instance of TargetsMetadata.
func InitializeTargetsMetadata() *tuf.TargetsMetadata {
	targetsMetadata := tuf.NewTargetsMetadata()
	targetsMetadata.SetExpires(time.Now().AddDate(1, 0, 0).Format(time.RFC3339))
	targetsMetadata.Delegations.AddDelegation(tuf.AllowRule())
	return targetsMetadata
}

// AddDelegation adds a new delegation to TargetsMetadata.
func AddDelegation(targetsMetadata *tuf.TargetsMetadata, ruleName string, authorizedKeys []*tuf.Key, rulePatterns []string, threshold int) (*tuf.TargetsMetadata, error) {
	if targetsMetadata == nil {
		return nil, ErrTargetsMetadataNil
	}

	if err := targetsMetadata.AddDelegation(ruleName, authorizedKeys, rulePatterns, threshold); err != nil {
		return nil, err
	}
	return targetsMetadata, nil
}

// UpdateDelegation is used to amend a delegation in TargetsMetadata.
func UpdateDelegation(targetsMetadata *tuf.TargetsMetadata, ruleName string, authorizedKeys []*tuf.Key, rulePatterns []string, threshold int) (*tuf.TargetsMetadata, error) {
	if targetsMetadata == nil {
		return nil, ErrTargetsMetadataNil
	}

	if err := targetsMetadata.UpdateDelegation(ruleName, authorizedKeys, rulePatterns, threshold); err != nil {
		return nil, err
	}
	return targetsMetadata, nil
}

// ReorderDelegations changes the order of delegations, and the new order is
// specified in `ruleNames []string`.
func ReorderDelegations(targetsMetadata *tuf.TargetsMetadata, ruleNames []string) (*tuf.TargetsMetadata, error) {
	if targetsMetadata == nil {
		return nil, ErrTargetsMetadataNil
	}

	if err := targetsMetadata.ReorderDelegations(ruleNames); err != nil {
		return nil, err
	}
	return targetsMetadata, nil
}

// RemoveDelegation deletes a delegation entry from TargetsMetadata.
func RemoveDelegation(targetsMetadata *tuf.TargetsMetadata, ruleName string) (*tuf.TargetsMetadata, error) {
	if targetsMetadata == nil {
		return nil, ErrTargetsMetadataNil
	}

	if err := targetsMetadata.RemoveDelegation(ruleName); err != nil {
		return nil, err
	}
	return targetsMetadata, nil
}

// AddKeyToTargets adds public keys to the specified targets metadata.
func AddKeyToTargets(targetsMetadata *tuf.TargetsMetadata, authorizedKeys []*tuf.Key) (*tuf.TargetsMetadata, error) {
	if targetsMetadata == nil {
		return nil, ErrTargetsMetadataNil
	}

	for _, key := range authorizedKeys {
		targetsMetadata.Delegations.AddKey(key)
	}
	return targetsMetadata, nil
}
