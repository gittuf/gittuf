// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/gittuf/gittuf/internal/attestations"
	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/gittuf/gittuf/internal/signerverifier"
	"github.com/gittuf/gittuf/internal/signerverifier/dsse"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/go-git/go-git/v5/plumbing"
	sslibdsse "github.com/secure-systems-lab/go-securesystemslib/dsse"
)

var ErrNotSigningKey = errors.New("expected signing key")

// AddReferenceAuthorization adds a reference authorization attestation to the
// repository for the specified ref. The from ID is identified using the RSL
// while the to ID is set to the current status of the ref.
func (r *Repository) AddReferenceAuthorization(ctx context.Context, keyBytes []byte, targetRef string, signCommit bool) error {
	targetRef, err := gitinterface.AbsoluteReference(r.r, targetRef)
	if err != nil {
		return err
	}

	allAttestations, err := attestations.LoadCurrentAttestations(r.r)
	if err != nil {
		return err
	}

	var (
		fromID string
		toID   string
	)

	latestEntry, _, err := rsl.GetLatestReferenceEntryForRef(r.r, targetRef)
	if err == nil {
		fromID = latestEntry.TargetID.String()
	} else {
		if !errors.Is(err, rsl.ErrRSLEntryNotFound) {
			return err
		}
		fromID = plumbing.ZeroHash.String()
	}

	ref, err := r.r.Reference(plumbing.ReferenceName(targetRef), true)
	if err != nil {
		return err
	}
	toID = ref.Hash().String()

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

	signer, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(keyBytes)
	if err != nil {
		return err
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
// their key.
func (r *Repository) RemoveReferenceAuthorization(keyBytes []byte, targetRef, fromID, toID string, signCommit bool) error {
	key, err := tuf.LoadKeyFromBytes(keyBytes)
	if err != nil {
		return err
	}
	if key.KeyVal.Private == "" {
		// The authorization removal process must be authenticated using the
		// original signing key
		return ErrNotSigningKey
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
		if signature.KeyID != key.KeyID {
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

	commitMessage := fmt.Sprintf("Remove reference authorization for '%s' from '%s' to '%s' by '%s'", targetRef, fromID, toID, key.KeyID)

	return allAttestations.Commit(r.r, commitMessage, signCommit)
}
