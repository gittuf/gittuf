// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tuf

import (
	"errors"
	"github.com/gittuf/gittuf/internal/gitinterface"

	"github.com/gittuf/gittuf/internal/common/set"
	"github.com/secure-systems-lab/go-securesystemslib/signerverifier"
)

const (
	// RootRoleName defines the expected name for the gittuf root of trust.
	RootRoleName = "root"

	// TargetsRoleName defines the expected name for the top level gittuf policy file.
	TargetsRoleName = "targets"

	// GitHubAppRoleName defines the expected name for the GitHub app role in the root of trust metadata.
	GitHubAppRoleName = "github-app"

	AllowRuleName = "gittuf-allow-rule"
)

var (
	ErrInvalidRootMetadata                      = errors.New("invalid root metadata")
	ErrPrimaryRuleFileInformationNotFoundInRoot = errors.New("root metadata does not contain primary rule file information")
	ErrGitHubAppInformationNotFoundInRoot       = errors.New("the special GitHub app role is not defined, but GitHub app approvals is set to trusted")
	ErrDuplicatedRuleName                       = errors.New("two rules with same name found in policy")
	ErrInvalidPrincipalID                       = errors.New("principal ID is invalid")
	ErrInvalidPrincipalType                     = errors.New("invalid principal type (do you have the right gittuf version?)")
	ErrRuleNotFound                             = errors.New("cannot find rule entry")
	ErrMissingRules                             = errors.New("some rules are missing")
	ErrCannotManipulateAllowRule                = errors.New("cannot change in-built gittuf-allow-rule")
	ErrCannotMeetThreshold                      = errors.New("insufficient keys to meet threshold")
)

// Principal represents an entity that is granted trust by gittuf metadata. In
// the simplest case, a principal may be a single public key. On the other hand,
// a principal may represent a human (who may control multiple keys), a team
// (consisting of multiple humans) etc.
type Principal interface {
	ID() string
	Keys() []*signerverifier.SSLibKey
}

// RootMetadata represents the root of trust metadata for gittuf.
type RootMetadata interface {
	// SetExpires sets the expiry time for the metadata.
	// TODO: Does expiry make sense for the gittuf context? This is currently
	// unenforced
	SetExpires(expiry string)

	// SchemaVersion returns the metadata schema version.
	SchemaVersion() string

	// GetPrincipals returns all the principals in the root metadata.
	GetPrincipals() map[string]Principal

	// AddRootPrincipal adds the corresponding principal to the root metadata
	// file and marks it as trusted for subsequent root of trust metadata.
	AddRootPrincipal(principal Principal) error
	// DeleteRootPrincipal removes the corresponding principal from the set of
	// trusted principals for the root of trust.
	DeleteRootPrincipal(principalID string) error
	// UpdateRootThreshold sets the required number of signatures for root of
	// trust metadata.
	UpdateRootThreshold(threshold int) error
	// GetRootPrincipals returns the principals trusted for the root of trust
	// metadata.
	GetRootPrincipals() ([]Principal, error)
	// GetRootThreshold returns the threshold of principals that must sign the
	// root of trust metadata.
	GetRootThreshold() (int, error)

	// AddPrincipalRuleFilePrincipal adds the corresponding principal to the
	// root metadata file and marks it as trusted for the primary rule file.
	AddPrimaryRuleFilePrincipal(principal Principal) error
	// DeletePrimaryRuleFilePrincipal removes the corresponding principal from
	// the set of trusted principals for the primary rule file.
	DeletePrimaryRuleFilePrincipal(principalID string) error
	// UpdatePrimaryRuleFileThreshold sets the required number of signatures for
	// the primary rule file.
	UpdatePrimaryRuleFileThreshold(threshold int) error
	// GetPrimaryRuleFilePrincipals returns the principals trusted for the
	// primary rule file.
	GetPrimaryRuleFilePrincipals() ([]Principal, error)
	// GetPrimaryRuleFileThreshold returns the threshold of principals that must
	// sign the primary rule file.
	GetPrimaryRuleFileThreshold() (int, error)

	// AddGitHubAppPrincipal adds the corresponding principal to the root
	// metadata and is trusted for GitHub app attestations.
	// TODO: this needs to be generalized across tools
	AddGitHubAppPrincipal(principal Principal) error
	// DeleteGitHubAppPrincipal removes the GitHub app attestations role from
	// the root of trust metadata.
	// TODO: this needs to be generalized across tools
	DeleteGitHubAppPrincipal()
	// EnableGitHubAppApprovals indicates attestations from the GitHub app role
	// must be trusted.
	// TODO: this needs to be generalized across tools
	EnableGitHubAppApprovals()
	// DisableGitHubAppApprovals indicates attestations from the GitHub app role
	// must not be trusted thereafter.
	// TODO: this needs to be generalized across tools
	DisableGitHubAppApprovals()
	// IsGitHubAppApprovalTrusted indicates if the GitHub app is trusted.
	// TODO: this needs to be generalized across tools
	IsGitHubAppApprovalTrusted() bool
	// GetGitHubAppPrincipals returns the principals trusted for the GitHub app
	// attestations.
	// TODO: this needs to be generalized across tools
	GetGitHubAppPrincipals() ([]Principal, error)
}

// TargetsMetadata represents gittuf's rule files. Its name is inspired by TUF.
type TargetsMetadata interface {
	// SetExpires sets the expiry time for the metadata.
	// TODO: Does expiry make sense for the gittuf context? This is currently
	// unenforced
	SetExpires(expiry string)

	SetHooksField(hooksID gitinterface.Hash)

	GetHooksField() any
	// SchemaVersion returns the metadata schema version.
	SchemaVersion() string

	// GetPrincipals returns all the principals in the rule file.
	GetPrincipals() map[string]Principal

	// GetRules returns all the rules in the metadata.
	GetRules() []Rule

	// AddRule adds a rule to the metadata file.
	AddRule(ruleName string, authorizedPrincipals []Principal, rulePatterns []string, threshold int) error
	// UpdateRule updates an existing rule identified by ruleName with the
	// provided parameters.
	UpdateRule(ruleName string, authorizedPrincipals []Principal, rulePatterns []string, threshold int) error
	// ReorderRules accepts the new order of rules (identified by their
	// ruleNames).
	ReorderRules(newRuleNames []string) error
	// RemoveRule deletes the rule identified by the ruleName.
	RemoveRule(ruleName string) error

	// AddPrincipal adds a principal to the metadata.
	// TODO: this isn't associated with a specific rule; with the removal of
	// verify-commit and verify-tag, it may not make sense anymore
	AddPrincipal(principal Principal) error
}

// Rule represents a rule entry in a rule file (`TargetsMetadata`).
type Rule interface {
	// ID returns the identifier of the rule, typically a name.
	ID() string

	// Matches indicates if the rule applies to a specified path.
	Matches(path string) bool

	// GetProtectedNamespaces returns the set of namespaces protected by the
	// rule.
	GetProtectedNamespaces() []string

	// GetPrincipalIDs returns the identifiers of the principals that are listed
	// as trusted by the rule.
	GetPrincipalIDs() *set.Set[string]
	// GetThreshold returns the threshold of principals that must approve to
	// meet the rule.
	GetThreshold() int

	// IsLastTrustedInRuleFile indicates that subsequent rules in the rule file
	// are not to be trusted if the current rule matches the namespace under
	// verification (similar to TUF's terminating behavior). However, the
	// current rule's delegated rules as well as other rules already in the
	// queue are trusted.
	IsLastTrustedInRuleFile() bool
}
