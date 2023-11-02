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
)

func (r *Repository) AddAuthorization(ctx context.Context, targetRefName, sourceRefName string, signingKeyBytes []byte, signCommit bool) error {
	sv, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(signingKeyBytes)
	if err != nil {
		return err
	}

	keyID, err := sv.KeyID()
	if err != nil {
		return err
	}

	absTargetRefName, err := gitinterface.AbsoluteReference(r.r, targetRefName)
	if err != nil {
		return err
	}
	absSourceRefName, err := gitinterface.AbsoluteReference(r.r, sourceRefName)
	if err != nil {
		return err
	}

	// TODO: get unskipped entry?
	fromEntry, _, err := rsl.GetLatestReferenceEntryForRef(r.r, absTargetRefName)
	if err != nil {
		return err
	}
	// TODO: get unskipped entry?
	toEntry, _, err := rsl.GetLatestReferenceEntryForRef(r.r, absSourceRefName)
	if err != nil {
		return err
	}

	// The current accepted state of the target ref is the from ID.
	// The source ref is a feature branch or other ref that contains the change
	// that must be applied to the target ref.
	// TODO: support merge commits
	fromID := fromEntry.TargetID.String()
	toID := toEntry.TargetID.String()

	currentAttestations, err := attestations.LoadCurrentAttestations(r.r)
	if err != nil {
		return err
	}

	env, err := currentAttestations.GetAuthorizationAttestationFor(r.r, absTargetRefName, fromID, toID)
	if err != nil && !errors.Is(err, attestations.ErrAuthorizationNotFound) {
		return err
	}

	if env == nil {
		authorization, err := attestations.NewAuthorizationAttestation(absTargetRefName, fromID, toID)
		if err != nil {
			return err
		}

		env, err = dsse.CreateEnvelope(authorization)
		if err != nil {
			return err
		}
	}

	env, err = dsse.SignEnvelope(ctx, env, sv)
	if err != nil {
		return err
	}

	if err := currentAttestations.AddAuthorizationAttestation(r.r, env, absTargetRefName, fromID, toID); err != nil {
		return err
	}

	commitMessage := fmt.Sprintf("Add authorization of %s to move %s from %s to %s", keyID, absTargetRefName, fromID, toID)
	return currentAttestations.Commit(r.r, commitMessage, signCommit)
}

func (r *Repository) RemoveAuthorization(ctx context.Context, refName, fromID, toID string, keyBytes []byte, signCommit bool) error {
	// We pass in the signing key because we want to be able to gate revocation
	// to the authorizing identity We need to think about this in the policy
	// context as well though
	sv, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(keyBytes)
	if err != nil {
		return err
	}

	// TODO: sslib should have a "CanSign" method for us to check if we actually
	// have a signer
	if _, err := sv.Sign(ctx, []byte{}); err != nil {
		return err
	}

	keyID, err := sv.KeyID()
	if err != nil {
		return err
	}

	absRefName, err := gitinterface.AbsoluteReference(r.r, refName)
	if err != nil {
		return err
	}

	currentAttestations, err := attestations.LoadCurrentAttestations(r.r)
	if err != nil {
		return err
	}

	if err := currentAttestations.RemoveAuthorization(r.r, absRefName, fromID, toID, keyID); err != nil {
		if errors.Is(err, attestations.ErrAuthorizationNotFound) {
			return nil
		}

		return err
	}

	commitMessage := fmt.Sprintf("Revoke %s's authorization to move %s from %s to %s", keyID, absRefName, fromID, toID)
	return currentAttestations.Commit(r.r, commitMessage, signCommit)
}
