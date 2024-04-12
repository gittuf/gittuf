// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/gittuf/gittuf/internal/attestations"
	"github.com/gittuf/gittuf/internal/dev"
	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/gittuf/gittuf/internal/signerverifier/dsse"
	"github.com/go-git/go-git/v5/plumbing"
	sslibdsse "github.com/secure-systems-lab/go-securesystemslib/dsse"
)

var ErrNotSigningKey = errors.New("expected signing key")

// AddReferenceAuthorization adds a reference authorization attestation to the
// repository for the specified target ref. The from ID is identified using the
// last RSL entry for the target ref. The to ID is that of the expected Git tree
// created by merging the feature ref into the target ref. The commit used to
// calculate the merge tree ID is identified using the RSL for the feature ref.
// Currently, this is limited to developer mode.
func (r *Repository) AddReferenceAuthorization(ctx context.Context, signer sslibdsse.SignerVerifier, targetRef, featureRef string, signCommit bool) error {
	if !dev.InDevMode() {
		return dev.ErrNotInDevMode
	}

	var err error

	targetRef, err = gitinterface.AbsoluteReference(r.r, targetRef)
	if err != nil {
		return err
	}

	featureRef, err = gitinterface.AbsoluteReference(r.r, featureRef)
	if err != nil {
		return err
	}

	allAttestations, err := attestations.LoadCurrentAttestations(r.r)
	if err != nil {
		return err
	}

	var (
		fromID          string
		featureCommitID string
		toID            string
	)

	latestTargetEntry, _, err := rsl.GetLatestReferenceEntryForRef(r.r, targetRef)
	if err == nil {
		fromID = latestTargetEntry.TargetID.String()
	} else {
		if !errors.Is(err, rsl.ErrRSLEntryNotFound) {
			return err
		}
		fromID = plumbing.ZeroHash.String()
	}

	latestFeatureEntry, _, err := rsl.GetLatestReferenceEntryForRef(r.r, featureRef)
	if err != nil {
		// We don't have an RSL entry for the feature ref to use to approve the
		// merge
		return err
	}
	featureCommitID = latestFeatureEntry.TargetID.String()

	mergeTreeID, err := gitinterface.GetMergeTree(r.r, fromID, featureCommitID)
	if err != nil {
		return err
	}
	toID = mergeTreeID

	// Does a reference authorization already exist for the parameters?
	hasAuthorization := false
	env, err := allAttestations.GetReferenceAuthorizationFor(r.r, targetRef, fromID, toID)
	if err == nil {
		hasAuthorization = true
	} else if !errors.Is(err, attestations.ErrAuthorizationNotFound) {
		return err
	}

	if !hasAuthorization {
		// Create a new reference authorization and embed in env
		statement, err := attestations.NewReferenceAuthorization(targetRef, fromID, toID)
		if err != nil {
			return err
		}

		env, err = dsse.CreateEnvelope(statement)
		if err != nil {
			return err
		}
	}

	env, err = dsse.SignEnvelope(ctx, env, signer)
	if err != nil {
		return err
	}

	if err := allAttestations.SetReferenceAuthorization(r.r, env, targetRef, fromID, toID); err != nil {
		return err
	}

	commitMessage := fmt.Sprintf("Add reference authorization for '%s' from '%s' to '%s'", targetRef, fromID, toID)

	return allAttestations.Commit(r.r, commitMessage, signCommit)
}

// RemoveReferenceAuthorization removes a previously issued authorization for
// the specified parameters. The issuer of the authorization is identified using
// their key. Currently, this is limited to developer mode.
func (r *Repository) RemoveReferenceAuthorization(ctx context.Context, signer sslibdsse.SignerVerifier, targetRef, fromID, toID string, signCommit bool) error {
	if !dev.InDevMode() {
		return dev.ErrNotInDevMode
	}

	// Ensure only the key that created a reference authorization can remove it
	_, err := signer.Sign(ctx, nil)
	if err != nil {
		return errors.Join(ErrNotSigningKey, err)
	}
	keyID, err := signer.KeyID()
	if err != nil {
		return err
	}

	targetRef, err = gitinterface.AbsoluteReference(r.r, targetRef)
	if err != nil {
		return err
	}

	allAttestations, err := attestations.LoadCurrentAttestations(r.r)
	if err != nil {
		return err
	}

	env, err := allAttestations.GetReferenceAuthorizationFor(r.r, targetRef, fromID, toID)
	if err != nil {
		if errors.Is(err, attestations.ErrAuthorizationNotFound) {
			// No reference authorization at all
			return nil
		}
		return err
	}

	newSignatures := []sslibdsse.Signature{}
	for _, signature := range env.Signatures {
		// This handles cases where the envelope may unintentionally have
		// multiple signatures from the same key
		if signature.KeyID != keyID {
			newSignatures = append(newSignatures, signature)
		}
	}

	if len(newSignatures) == 0 {
		// No signatures, we can remove the ReferenceAuthorization altogether
		if err := allAttestations.RemoveReferenceAuthorization(targetRef, fromID, toID); err != nil {
			return err
		}
	} else {
		// We still have other signatures, so set the ReferenceAuthorization
		// envelope
		env.Signatures = newSignatures
		if err := allAttestations.SetReferenceAuthorization(r.r, env, targetRef, fromID, toID); err != nil {
			return err
		}
	}

	commitMessage := fmt.Sprintf("Remove reference authorization for '%s' from '%s' to '%s' by '%s'", targetRef, fromID, toID, keyID)

	return allAttestations.Commit(r.r, commitMessage, signCommit)
}
