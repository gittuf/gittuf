// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package slsa

import (
	"fmt"

	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/gittuf/gittuf/internal/version"
	"github.com/gittuf/gittuf/pkg/githash"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	vsa "github.com/in-toto/attestation/go/predicates/vsa/v1"
	ita "github.com/in-toto/attestation/go/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	verifierIDFormatter              = "https://gittuf.dev/verifier/%s"
	verificationSummaryPredicateType = "https://slsa.dev/verification_summary/v1"
	gitCommitDigestType              = "gitCommit"
	passedVSAStatus                  = "PASSED"
	sourceBranchesAnnotationType     = "source_branches"
)

type slsaSourceLevel uint

const (
	sourceLevel1 slsaSourceLevel = iota + 1
	sourceLevel2
	sourceLevel3
	sourceLevel4
)

const (
	sourceLevel1String = "SLSA_SOURCE_LEVEL_1"
	sourceLevel2String = "SLSA_SOURCE_LEVEL_2"
	sourceLevel3String = "SLSA_SOURCE_LEVEL_3"
	sourceLevel4String = "SLSA_SOURCE_LEVEL_4"
)

func (l slsaSourceLevel) String() string {
	switch l {
	case sourceLevel1:
		return sourceLevel1String
	case sourceLevel2:
		return sourceLevel2String
	case sourceLevel3:
		return sourceLevel3String
	case sourceLevel4:
		return sourceLevel4String
	default:
		return ""
	}
}

func slsaSourceLevelFromString(s string) slsaSourceLevel {
	switch s {
	case sourceLevel1String:
		return sourceLevel1
	case sourceLevel2String:
		return sourceLevel2
	case sourceLevel3String:
		return sourceLevel3
	case sourceLevel4String:
		return sourceLevel4
	default:
		return 0
	}
}

func GenerateGranularVSAs(verificationReport *policy.VerificationReport, repositoryLocation string) ([]*ita.Statement, error) {
	generators := []*vsaGenerator{}

	for index, entryVerificationReport := range verificationReport.EntryVerificationReports {
		// As we process the entry verification reports, we determine when and
		// how many VSAs to generate. We populate VSA generators with the
		// appropriate information, and generate all VSAs later.
		if len(generators) == 0 {
			// This is the very start of verification
			generators = append(generators, &vsaGenerator{
				policyID:   entryVerificationReport.PolicyID,
				revisionID: entryVerificationReport.TargetID,
				startIndex: 0, // hardcoded because this is the first, we also don't need this because it's the default, but more readable!
			})
		} else {
			// We have at least one generator
			// We have to update the existing generator **unless the policy has
			// now changed**

			if entryVerificationReport.PolicyID.Equal(generators[len(generators)-1].policyID) {
				// The policy has not changed, so we just update the revision ID
				generators[len(generators)-1].revisionID = entryVerificationReport.TargetID
			} else {
				// The policy has changed!
				// So, first we set the end conditions for the last generator,
				// then we add a new generator for subsequent entries

				// Note that only endIndex must be set
				// startIndex is set when the generator is added, as is the policyID
				// the revisionID is not set here because the last entry that
				// used the policy already has that set when there was no policy
				// change
				generators[len(generators)-1].endIndex = index // we don't use index-1 because we use this for range constraints later

				generators = append(generators, &vsaGenerator{
					policyID:   entryVerificationReport.PolicyID,
					revisionID: entryVerificationReport.TargetID,
					startIndex: index,
				})
			}
		}
	}

	// We must update the end conditions for the last generator
	generators[len(generators)-1].endIndex = len(verificationReport.EntryVerificationReports)

	// Now, for each generator, we produce the corresponding attestation
	allAttestations := []*ita.Statement{}
	for _, generator := range generators {
		statement, err := generator.generate(repositoryLocation, verificationReport.RefName, verificationReport.EntryVerificationReports[generator.startIndex:generator.endIndex])
		if err != nil {
			return nil, err
		}
		allAttestations = append(allAttestations, statement)
	}

	return allAttestations, nil
}

func GenerateMetaVSAFromGranularVSAs(granularVSAs []*ita.Statement, repositoryLocation string) (*ita.Statement, error) {
	// The meta VSA's policy is set to the applicable policy at the current revision
	// However, SLSA source level is set to whatever is the lowest of all VSAs
	// due to the current revision being built on top of the older changes that
	// didn't conform with higher levels

	sourceLevel := sourceLevel4
	predicate := &vsa.VerificationSummary{}
	for _, statement := range granularVSAs {
		predicateBytes, err := protojson.Marshal(statement.Predicate)
		if err != nil {
			return nil, err
		}

		predicate = &vsa.VerificationSummary{}
		if err := protojson.Unmarshal(predicateBytes, predicate); err != nil {
			return nil, err
		}

		predicateSourceLevel := slsaSourceLevelFromString(predicate.VerifiedLevels[0])
		sourceLevel = min(predicateSourceLevel, sourceLevel)
	}

	policyID, _ := githash.NewHash(predicate.Policy.Digest[gitCommitDigestType])

	lastVSAStatement := granularVSAs[len(granularVSAs)-1]
	revisionID, _ := gitinterface.NewHash(lastVSAStatement.Subject[0].Digest[gitCommitDigestType])
	refName := lastVSAStatement.Subject[0].Annotations.AsMap()[sourceBranchesAnnotationType].(string)

	generator := &vsaGenerator{policyID: policyID, revisionID: revisionID}
	return generator.generateWithSourceLevel(repositoryLocation, refName, sourceLevel)
}

type vsaGenerator struct {
	policyID   githash.Hash
	revisionID githash.Hash

	startIndex int
	endIndex   int
}

func (v *vsaGenerator) generate(repositoryLocation, refName string, entryVerificationReports []*policy.EntryVerificationReport) (*ita.Statement, error) {
	sourceLevel := v.identifySourceLevel(entryVerificationReports)
	return v.generateWithSourceLevel(repositoryLocation, refName, sourceLevel)
}

func (v *vsaGenerator) generateWithSourceLevel(repositoryLocation, refName string, sourceLevel slsaSourceLevel) (*ita.Statement, error) {
	predicate := &vsa.VerificationSummary{
		Verifier: &vsa.VerificationSummary_Verifier{
			Id: generateVerifierID(),
		},
		TimeVerified: timestamppb.Now(),
		ResourceUri:  repositoryLocation,
		Policy: &vsa.VerificationSummary_Policy{
			Digest: map[string]string{
				gitCommitDigestType: v.policyID.String(),
			},
		},
		VerificationResult: passedVSAStatus, // hardcoded, not generating VSA on failure
		VerifiedLevels:     []string{sourceLevel.String()},
	}

	predicateBytes, err := protojson.Marshal(predicate)
	if err != nil {
		return nil, err
	}

	predicateStruct := &structpb.Struct{}
	if err := protojson.Unmarshal(predicateBytes, predicateStruct); err != nil {
		return nil, err
	}

	annotations, err := structpb.NewStruct(map[string]any{sourceBranchesAnnotationType: refName})
	if err != nil {
		return nil, err
	}

	return &ita.Statement{
		Type: ita.StatementTypeUri,
		Subject: []*ita.ResourceDescriptor{
			{
				Digest: map[string]string{
					gitCommitDigestType: v.revisionID.String(),
				},
				Annotations: annotations,
			},
		},
		PredicateType: verificationSummaryPredicateType,
		Predicate:     predicateStruct,
	}, nil
}

func (v *vsaGenerator) identifySourceLevel(entryVerificationReports []*policy.EntryVerificationReport) slsaSourceLevel {
	// 1. Do ALL entry verification reports have block-force-pushes type global
	// rule?
	// Yes -> At least Level 2, next step.
	// No -> it's Level 1.

	// 2. Do ALL entry verification reports have a policy on them? (e.g. not
	// turned off for one commit and then turned back on)
	// Yes -> At least Level 3, next step.
	// No -> it's Level 2.

	// 3. Do ALL entry verification reports have TWO TRUSTED users approving
	// changes?
	// Yes -> it's Level 4.
	// No -> it's Level 3.

	sourceLevel := sourceLevel4
	hasPolicy := false
	for _, report := range entryVerificationReports {
		// 1
		// Iterate over the global rules present. If we find a
		// block-force-pushes rule, we're happy.
		hasBlockForcePushes := false
		for _, globalRule := range report.GlobalRuleVerificationReports {
			if globalRule.RuleType == tuf.GlobalRuleBlockForcePushesType {
				hasBlockForcePushes = true
			}
		}

		if !hasBlockForcePushes {
			// All reports must have this, and so we can return early here as
			// it's Level 1
			return sourceLevel1
		}

		// 2
		// Check if a rule was used to verify this change. Once a rule is used,
		// all future reports must also verify these changes with a rule, or
		// otherwise the policy has been disabled
		if !hasPolicy && report.RuleName != "" {
			hasPolicy = true
		} else if hasPolicy && report.RuleName == "" {
			sourceLevel = sourceLevel2
		}

		// 3
		// Check if the report has at least two accepted principal IDs.
		if report.AcceptedPrincipalIDs.Len() < 2 {
			sourceLevel = sourceLevel3
		}
	}

	return sourceLevel
}

func generateVerifierID() string {
	return fmt.Sprintf(verifierIDFormatter, version.GetVersion())
}
