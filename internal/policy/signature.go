// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/gittuf/gittuf/internal/common/set"
	"github.com/gittuf/gittuf/internal/signerverifier/common"
	"github.com/gittuf/gittuf/internal/signerverifier/dsse"
	"github.com/gittuf/gittuf/internal/signerverifier/gpg"
	"github.com/gittuf/gittuf/internal/signerverifier/sigstore"
	sigstoreverifieropts "github.com/gittuf/gittuf/internal/signerverifier/sigstore/options/verifier"
	"github.com/gittuf/gittuf/internal/signerverifier/ssh"
	sslibdsse "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/secure-systems-lab/go-securesystemslib/signerverifier"
)

type SignatureVerifier struct {
	repository         *gitinterface.Repository
	name               string
	principals         []tuf.Principal
	threshold          int
	verifyExhaustively bool // verifyExhaustively checks all possible signatures and returns all matched principals, even if threshold is already met
}

func (v *SignatureVerifier) Name() string {
	return v.name
}

func (v *SignatureVerifier) Threshold() int {
	return v.threshold
}

func (v *SignatureVerifier) TrustedPrincipalIDs() *set.Set[string] {
	principalIDs := set.NewSet[string]()
	for _, principal := range v.principals {
		principalIDs.Add(principal.ID())
	}

	return principalIDs
}

// Verify is used to check for a threshold of signatures using the verifier. The
// threshold of signatures may be met using a combination of at most one Git
// signature and signatures embedded in a DSSE envelope. Verify does not inspect
// the envelope's payload, but instead only verifies the signatures. The caller
// must ensure the validity of the envelope's contents.
func (v *SignatureVerifier) Verify(ctx context.Context, gitObjectID gitinterface.Hash, env *sslibdsse.Envelope) (*set.Set[string], error) {
	if v.threshold < 1 || len(v.principals) < 1 {
		return nil, ErrInvalidVerifier
	}

	// usedPrincipalIDs is ultimately returned to track the set of principals
	// who have been authenticated
	usedPrincipalIDs := set.NewSet[string]()

	// usedKeyIDs is tracked to ensure a key isn't duplicated between two
	// principals, allowing two principals to meet a threshold using the same
	// key
	usedKeyIDs := set.NewSet[string]()

	// gitObjectVerified is set to true if the gitObjectID's signature is
	// verified
	gitObjectVerified := false

	// First, verify the gitObject's signature if one is presented
	if gitObjectID != nil && !gitObjectID.IsZero() {
		slog.Debug(fmt.Sprintf("Verifying signature of Git object with ID '%s'...", gitObjectID.String()))
		for _, principal := range v.principals {
			// there are multiple keys we must try
			keys := principal.Keys()

			for _, key := range keys {
				err := v.repository.VerifySignature(ctx, gitObjectID, key)
				if err == nil {
					// Signature verification succeeded
					slog.Debug(fmt.Sprintf("Public key '%s' belonging to principal '%s' successfully used to verify signature of Git object '%s', counting '%s' towards threshold...", key.KeyID, principal.ID(), gitObjectID.String(), principal.ID()))
					usedPrincipalIDs.Add(principal.ID())
					usedKeyIDs.Add(key.KeyID)
					gitObjectVerified = true

					// No need to try the other keys for this principal, break
					break
				}
				if errors.Is(err, gitinterface.ErrUnknownSigningMethod) {
					// TODO: this should be removed once we have unified signing
					// methods across metadata and git signatures
					continue
				}
				if !errors.Is(err, gitinterface.ErrIncorrectVerificationKey) {
					return nil, err
				}
			}

			if gitObjectVerified {
				// No need to try other principals, break
				break
			}
		}
	}

	// If we don't have to verify exhaustively and threshold is 1 and the Git
	// signature is verified, we can return
	if !v.verifyExhaustively && v.threshold == 1 && gitObjectVerified {
		return usedPrincipalIDs, nil
	}

	slog.Debug("Proceeding with verification of attestations...")

	if env != nil {
		// Second, verify signatures on the envelope

		// We have to verify the envelope independently for each principal
		// trusted in the verifier as a principal may have multiple keys
		// associated with them.
		for _, principal := range v.principals {
			if usedPrincipalIDs.Has(principal.ID()) {
				// Do not verify using this principal as they were verified for
				// the Git signature
				slog.Debug(fmt.Sprintf("Principal '%s' has already been counted towards the threshold, skipping...", principal.ID()))
				continue
			}

			principalVerifiers := []sslibdsse.Verifier{}

			keys := principal.Keys()
			for _, key := range keys {
				if usedKeyIDs.Has(key.KeyID) {
					// this key has been encountered before, possibly because
					// another Principal included this key
					slog.Debug(fmt.Sprintf("Key with ID '%s' has already been used to verify a signature, skipping...", key.KeyID))
					continue
				}

				var (
					dsseVerifier sslibdsse.Verifier
					err          error
				)
				switch key.KeyType {
				case ssh.KeyType:
					slog.Debug(fmt.Sprintf("Found SSH key '%s'...", key.KeyID))
					dsseVerifier, err = ssh.NewVerifierFromKey(key)
					if err != nil {
						return nil, err
					}
				case gpg.KeyType:
					slog.Debug(fmt.Sprintf("Found GPG key '%s'...", key.KeyID))
					dsseVerifier, err = gpg.NewVerifierFromKey(key)
					if err != nil {
						return nil, err
					}
				case sigstore.KeyType:
					slog.Debug(fmt.Sprintf("Found Sigstore key '%s'...", key.KeyID))
					opts := []sigstoreverifieropts.Option{}
					config, err := v.repository.GetGitConfig()
					if err != nil {
						return nil, err
					}
					if rekorURL, has := config[sigstore.GitConfigRekor]; has {
						slog.Debug(fmt.Sprintf("Using '%s' as Rekor server...", rekorURL))
						opts = append(opts, sigstoreverifieropts.WithRekorURL(rekorURL))
					}

					dsseVerifier = sigstore.NewVerifierFromIdentityAndIssuer(key.KeyVal.Identity, key.KeyVal.Issuer, opts...)
				case signerverifier.ED25519KeyType:
					// These are only used to verify old policy metadata signed before the ssh-signer was added
					slog.Debug(fmt.Sprintf("Found legacy ED25519 key '%s' in custom securesystemslib format...", key.KeyID))
					dsseVerifier, err = signerverifier.NewED25519SignerVerifierFromSSLibKey(key)
					if err != nil {
						return nil, err
					}
				case signerverifier.RSAKeyType:
					// These are only used to verify old policy metadata signed before the ssh-signer was added
					slog.Debug(fmt.Sprintf("Found legacy RSA key '%s' in custom securesystemslib format...", key.KeyID))
					dsseVerifier, err = signerverifier.NewRSAPSSSignerVerifierFromSSLibKey(key)
					if err != nil {
						return nil, err
					}
				case signerverifier.ECDSAKeyType:
					// These are only used to verify old policy metadata signed before the ssh-signer was added
					slog.Debug(fmt.Sprintf("Found legacy ECDSA key '%s' in custom securesystemslib format...", key.KeyID))
					dsseVerifier, err = signerverifier.NewECDSASignerVerifierFromSSLibKey(key)
					if err != nil {
						return nil, err
					}
				default:
					return nil, common.ErrUnknownKeyType
				}

				principalVerifiers = append(principalVerifiers, dsseVerifier)
			}

			// We have the principal's verifiers: use that to verify the envelope
			if len(principalVerifiers) == 0 {
				// TODO: remove this when we have signing method unification
				// across git and dsse
				continue
			}

			// We set threshold to 1 as we only need one of the keys for this
			// principal to be matched. If more than one key is matched and
			// returned in acceptedKeys, we count this only once towards the
			// principal and therefore the verifier's threshold. However, for
			// safety, we count both keys. If two principals share keys, this
			// can lead to a problem meeting thresholds. Arguably, they
			// shouldn't be sharing keys, so this seems reasonable.
			acceptedKeys, err := dsse.VerifyEnvelope(ctx, env, principalVerifiers, 1)
			if err != nil && !strings.Contains(err.Error(), "accepted signatures do not match threshold") {
				return nil, err
			}

			for _, key := range acceptedKeys {
				// Mark all accepted keys as used: this doesn't count towards
				// the threshold directly, but if another principal has the same
				// key, they may not be counted towards the threshold
				slog.Debug(fmt.Sprintf("Public key '%s' belonging to principal '%s' successfully used to verify signature of attestation, counting '%s' towards threshold...", key.KeyID, principal.ID(), principal.ID()))
				usedKeyIDs.Add(key.KeyID)
				usedPrincipalIDs.Add(principal.ID())
			}
		}
	}

	if v.verifyExhaustively || usedPrincipalIDs.Len() >= v.Threshold() {
		// TODO: double check that this is okay!
		return usedPrincipalIDs, nil
	}

	// Return usedPrincipalIDs so the consumer can decide what to do with the
	// principals that were used
	return usedPrincipalIDs, ErrVerifierConditionsUnmet
}
