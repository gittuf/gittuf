// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gittuf

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strings"

	verifyopts "github.com/gittuf/gittuf/experimental/gittuf/options/verify"
	verifymergeableopts "github.com/gittuf/gittuf/experimental/gittuf/options/verifymergeable"
	"github.com/gittuf/gittuf/internal/attestations"
	"github.com/gittuf/gittuf/internal/attestations/slsa"
	"github.com/gittuf/gittuf/internal/dev"
	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/gittuf/gittuf/internal/signerverifier/dsse"
)

// ErrRefStateDoesNotMatchRSL is returned when a Git reference being verified
// does not have the same tip as identified in the latest RSL entry for the
// reference. This can happen for a number of reasons such as incorrectly
// modifying reference state away from what's recorded in the RSL to not
// creating an RSL entry for some new changes. Depending on the context, one
// resolution is to update the reference state to match the RSL entry, while
// another is to create a new RSL entry for the current state.
var ErrRefStateDoesNotMatchRSL = errors.New("current state of Git reference does not match latest RSL entry")

func (r *Repository) VerifyRef(ctx context.Context, refName string, opts ...verifyopts.Option) error {
	var (
		verificationReport *policy.VerificationReport
		err                error
	)

	options := &verifyopts.Options{}
	for _, fn := range opts {
		fn(options)
	}

	slog.Debug("Identifying absolute reference path...")
	refName, err = r.r.AbsoluteReference(refName)
	if err != nil {
		return err
	}

	// Track localRefName to check the expected tip as we may override refName
	localRefName := refName

	if options.RefNameOverride != "" {
		// remote ref name is different
		// We must consider RSL entries that have refNameOverride rather than
		// refName
		slog.Debug("Name of reference overridden to match remote reference name, identifying absolute reference path...")
		refNameOverride, err := r.r.AbsoluteReference(options.RefNameOverride)
		if err != nil {
			return err
		}

		refName = refNameOverride
	}

	slog.Debug(fmt.Sprintf("Verifying gittuf policies for '%s'", refName))

	verifier := policy.NewPolicyVerifier(r.r)

	if options.LatestOnly {
		verificationReport, err = verifier.VerifyRef(ctx, refName)
	} else {
		verificationReport, err = verifier.VerifyRefFull(ctx, refName)
	}
	if err != nil {
		return err
	}

	// To verify the tip, we _must_ use the localRefName
	slog.Debug("Verifying if tip of reference matches expected value from RSL...")
	if err := r.verifyRefTip(localRefName, verificationReport.ExpectedTip); err != nil {
		return err
	}

	slog.Debug("Verification successful!")

	if options.GranularVSAsPath != "" || options.MetaVSAPath != "" {
		slog.Debug("Generating verification summary attestation(s)...")

		latestPolicy, err := policy.LoadCurrentState(ctx, r.r, policy.PolicyRef)
		if err != nil {
			return err
		}
		rootMetadata, err := latestPolicy.GetRootMetadata(false)
		if err != nil {
			return err
		}

		allAttestations, err := slsa.GenerateGranularVSAs(r.r, verificationReport, rootMetadata.GetRepositoryLocation())
		if err != nil {
			return err
		}

		if options.GranularVSAsPath != "" {
			// Write attestation bundle to disk
			envs := []string{}

			for _, attestation := range allAttestations {
				env, err := dsse.CreateEnvelope(attestation)
				if err != nil {
					return err
				}

				if options.VSASigner != nil {
					env, err = dsse.SignEnvelope(ctx, env, options.VSASigner)
					if err != nil {
						return err
					}
				}

				envBytes, err := json.Marshal(env)
				if err != nil {
					return err
				}
				envs = append(envs, string(envBytes))
			}

			jsonLines := strings.Join(envs, "\n")
			jsonLines += "\n"
			if err := os.WriteFile(options.GranularVSAsPath, []byte(jsonLines), 0o600); err != nil {
				return fmt.Errorf("error writing attestation bundle of all VSAs: %w", err)
			}
		}

		if options.MetaVSAPath != "" {
			metaVSA, err := slsa.GenerateMetaVSAFromGranularVSAs(r.r, allAttestations, rootMetadata.GetRepositoryLocation())
			if err != nil {
				return err
			}

			env, err := dsse.CreateEnvelope(metaVSA)
			if err != nil {
				return err
			}

			if options.VSASigner != nil {
				env, err = dsse.SignEnvelope(ctx, env, options.VSASigner)
				if err != nil {
					return err
				}
			}

			envBytes, err := json.Marshal(env)
			if err != nil {
				return err
			}

			if err := os.WriteFile(options.MetaVSAPath, envBytes, 0o600); err != nil {
				return fmt.Errorf("error writing meta VSA: %w", err)
			}
		}

		if options.SourceProvenanceBundlePath != "" {
			// Find last entry verification report
			entryVerificationReport := verificationReport.EntryVerificationReports[len(verificationReport.EntryVerificationReports)-1]

			// The attestations we care about are in the entry's field. In
			// addition, we need the merge attestation.
			sourceProvenanceAttestations := entryVerificationReport.ReferenceAuthorizations

			attestationsEntry, _, err := rsl.GetLatestReferenceUpdaterEntry(r.r, rsl.BeforeEntryID(entryVerificationReport.EntryID), rsl.ForReference(attestations.Ref))
			if err != nil {
				return fmt.Errorf("unable to fetch source provenance attestations: %w", err)
			}
			attestationsState, err := attestations.LoadAttestationsForEntry(r.r, attestationsEntry)
			if err != nil {
				return fmt.Errorf("unable to fetch source provenance attestations: %w", err)
			}

			baseInfo, err := getBaseInfoFromRepository(ctx, rootMetadata.GetRepositoryLocation())
			if err != nil {
				return fmt.Errorf("unable to fetch base repository information for source provenance: %w", err)
			}

			mergeAttestation, err := attestationsState.GetGitHubPullRequestAttestation(r.r, baseInfo, entryVerificationReport.RefName, entryVerificationReport.TargetID.String())
			if err != nil {
				return fmt.Errorf("unable to fetch source provenance merge attestations: %w", err)
			}
			sourceProvenanceAttestations = append(sourceProvenanceAttestations, mergeAttestation)

			envs := []string{}
			for _, attestation := range sourceProvenanceAttestations {
				envBytes, err := json.Marshal(attestation)
				if err != nil {
					return fmt.Errorf("unable to prepare source provenance payload: %w", err)
				}

				envs = append(envs, string(envBytes))
			}

			jsonLines := strings.Join(envs, "\n")
			jsonLines += "\n"
			if err := os.WriteFile(options.SourceProvenanceBundlePath, []byte(jsonLines), 0o600); err != nil {
				return fmt.Errorf("error writing attestation bundle of source provenance: %w", err)
			}
		}
	}

	return nil
}

func (r *Repository) VerifyRefFromEntry(ctx context.Context, refName, entryID string, opts ...verifyopts.Option) error {
	if !dev.InDevMode() {
		return dev.ErrNotInDevMode
	}

	options := &verifyopts.Options{}
	for _, fn := range opts {
		fn(options)
	}

	var err error

	slog.Debug("Identifying absolute reference path...")
	refName, err = r.r.AbsoluteReference(refName)
	if err != nil {
		return err
	}

	entryIDHash, err := gitinterface.NewHash(entryID)
	if err != nil {
		return err
	}

	// Track localRefName to check the expected tip as we may override refName
	localRefName := refName

	if options.RefNameOverride != "" {
		// remote ref name is different
		// We must consider RSL entries that have refNameOverride rather than
		// refName
		slog.Debug("Name of reference overridden to match remote reference name, identifying absolute reference path...")
		refNameOverride, err := r.r.AbsoluteReference(options.RefNameOverride)
		if err != nil {
			return err
		}

		refName = refNameOverride
	}

	slog.Debug(fmt.Sprintf("Verifying gittuf policies for '%s' from entry '%s'", refName, entryID))
	verifier := policy.NewPolicyVerifier(r.r)
	verificationReport, err := verifier.VerifyRefFromEntry(ctx, refName, entryIDHash)
	if err != nil {
		return err
	}

	// To verify the tip, we _must_ use the localRefName
	slog.Debug("Verifying if tip of reference matches expected value from RSL...")
	if err := r.verifyRefTip(localRefName, verificationReport.ExpectedTip); err != nil {
		return err
	}

	slog.Debug("Verification successful!")
	return nil
}

// VerifyMergeable checks if the targetRef can be updated to reflect the changes
// in featureRef. It checks if sufficient authorizations / approvals exist for
// the merge to happen, indicated by the error being nil. Additionally, a
// boolean value is also returned that indicates whether a final authorized
// signature is still necessary via the RSL entry for the merge.
//
// Summary of return combinations:
// (false, err) -> merge is not possible
// (false, nil) -> merge is possible and can be performed by anyone
// (true,  nil) -> merge is possible but it MUST be performed by an authorized
// person for the rule, i.e., an authorized person must sign the merge's RSL
// entry
func (r *Repository) VerifyMergeable(ctx context.Context, targetRef, featureRef string, opts ...verifymergeableopts.Option) (bool, error) {
	var err error

	options := &verifymergeableopts.Options{}
	for _, fn := range opts {
		fn(options)
	}

	slog.Debug("Identifying absolute reference paths...")
	targetRef, err = r.r.AbsoluteReference(targetRef)
	if err != nil {
		return false, err
	}
	featureRef, err = r.r.AbsoluteReference(featureRef)
	if err != nil {
		return false, err
	}

	slog.Debug(fmt.Sprintf("Inspecting gittuf policies to identify if '%s' can be merged into '%s' with current approvals...", featureRef, targetRef))
	verifier := policy.NewPolicyVerifier(r.r)

	var needRSLSignature bool

	if options.BypassRSLForFeatureRef {
		slog.Debug("Not using RSL for feature ref...")
		featureID, err := r.r.GetReference(featureRef)
		if err != nil {
			return false, err
		}

		needRSLSignature, err = verifier.VerifyMergeableForCommit(ctx, targetRef, featureID)
		if err != nil {
			return false, err
		}
	} else {
		needRSLSignature, err = verifier.VerifyMergeable(ctx, targetRef, featureRef)
		if err != nil {
			return false, err
		}
	}

	if needRSLSignature {
		slog.Debug("Merge is allowed but must be performed by authorized user who has not already issued an approval!")
	} else {
		slog.Debug("Merge is allowed and can be performed by any user!")
	}

	return needRSLSignature, nil
}

func (r *Repository) VerifyNetwork(ctx context.Context) error {
	verifier := policy.NewPolicyVerifier(r.r)
	return verifier.VerifyNetwork(ctx)
}

// verifyRefTip inspects the specified reference in the local repository to
// check if it points to the expected Git object.
func (r *Repository) verifyRefTip(target string, expectedTip gitinterface.Hash) error {
	refTip, err := r.r.GetReference(target)
	if err != nil {
		return err
	}

	if !refTip.Equal(expectedTip) {
		return ErrRefStateDoesNotMatchRSL
	}

	return nil
}

func getBaseInfoFromRepository(ctx context.Context, location string) (string, error) {
	// Return <username>-<id> for the entity that owns the repository
	// Say location is https://github.com/gittuf/gittuf
	u, err := url.Parse(location)
	if err != nil {
		return "", err
	}

	baseLocation := fmt.Sprintf("https://%s", u.Host)

	token := os.Getenv("GITHUB_TOKEN")

	client, err := getGitHubClient(baseLocation, token)
	if err != nil {
		return "", err
	}

	// u.Path is gittuf/gittuf from our example above
	split := strings.Split(u.Path, "/")

	repo, _, err := client.Repositories.Get(ctx, split[0], split[1])
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s-%d", repo.GetOwner().GetLogin(), repo.GetOwner().GetID()), nil
}
