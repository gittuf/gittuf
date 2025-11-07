// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"github.com/gittuf/gittuf/internal/common/set"
	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
	"github.com/gittuf/gittuf/internal/tuf"
)

type VerificationReport struct {
	ExpectedTip gitinterface.Hash
	RefName     string

	FirstRSLEntryVerified gitinterface.Hash
	LastRSLEntryVerified  gitinterface.Hash

	EntryVerificationReports []*EntryVerificationReport
}

type EntryVerificationReport struct {
	// First, record some meta information about the entry that was verified.

	// EntryID indicates the ID of the RSL reference entry that was verified.
	EntryID gitinterface.Hash
	// PolicyID indicates the ID of the RSL reference entry for the policy used
	// during verification.
	PolicyID gitinterface.Hash
	// RefName indicates the reference the verified entry is for.
	RefName string
	// TargetID indicates the target recorded in the verified entry.
	TargetID gitinterface.Hash

	// Second, record specific information about the entry verification result.

	// AcceptedPrincipalIDs indicates the principal IDs that were identified as
	// having approved the entry. This may be via the signature on the RSL entry
	// itself, one or more signatures on reference authorization attestations,
	// or via code review tool attestations (e.g., GitHub associated identity
	// for a person).
	AcceptedPrincipalIDs *set.Set[string]
	// RuleName indicates the name of the rule used to verify the entry. This is
	// only populated if an explicit rule is used during verification. In other
	// words, global rules don't go here.
	RuleName string
	// ReferenceAuthorization records the DSSE envelope for the reference
	// authorization identified as approving the entry.
	// TODO: should we capture the full envelope or just its blobID?
	ReferenceAuthorization *dsse.Envelope

	// Third, record information about the commit(s) verification result.

	CommitVerificationReports []*CommitVerificationReport

	// Fourth, record information about the global rules enforced.

	GlobalRuleVerificationReports []*GlobalRuleVerificationReport
}

type CommitVerificationReport struct {
	// First, record some meta information about the commit that was verified.

	// CommitID indicates the ID of the commit that is verified.
	CommitID gitinterface.Hash

	// Second, record specific information about the commit verification result.

	FileVerificationReports []*FileVerificationReport
}

type FileVerificationReport struct {
	FilePath string

	// AcceptedPrincipalIDs indicates the principal IDs that were identified as
	// having approved the entry. This may be via the signature on the RSL entry
	// itself, one or more signatures on reference authorization attestations,
	// or via code review tool attestations (e.g., GitHub associated identity
	// for a person).
	AcceptedPrincipalIDs *set.Set[string]
	// RuleName indicates the name of the rule used to verify the entry. This is
	// only populated if an explicit rule is used during verification. In other
	// words, global rules don't go here.
	RuleName string

	GlobalRuleVerificationReports []*GlobalRuleVerificationReport
}

type GlobalRuleVerificationReport struct {
	GlobalRule tuf.GlobalRule
	RuleName   string
	RuleType   string
}
