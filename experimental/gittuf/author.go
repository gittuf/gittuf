// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gittuf

import (
	"context"
	"crypto"
	"fmt"
	"time"

	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/signerverifier/dsse"
	"github.com/gittuf/gittuf/internal/signerverifier/ssh"
	sslibdsse "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
	"github.com/gittuf/gittuf/internal/tuf"
	tufv02 "github.com/gittuf/gittuf/internal/tuf/v02"
	"github.com/gittuf/gittuf/pkg/githash"
	"github.com/gittuf/gittuf/pkg/gitstore"
)

// authorPolicyRuleName names the top-level rule authorizing the RSL signers
// over the requested reference patterns.
const authorPolicyRuleName = "authorize-rsl-signers"

// AuthorPolicyParams configures AuthorPolicy. Keys are standard library types,
// so a caller outside this module can fill them in.
type AuthorPolicyParams struct {
	// RootSigners sign the root and top-level targets metadata. Each signer's
	// public key must be among RootPublicKeys, and there must be at least
	// RootThreshold distinct signers.
	RootSigners []crypto.Signer

	// RootPublicKeys are trusted for the root role and for the top-level
	// targets role.
	RootPublicKeys []crypto.PublicKey

	// RootThreshold is the number of signatures required for the root and
	// top-level targets roles.
	RootThreshold int

	// RSLSignerPublicKeys are authorized to sign RSL entries for RefPatterns.
	RSLSignerPublicKeys []crypto.PublicKey

	// RSLSignerThreshold is the number of RSL signatures required for a
	// reference matching RefPatterns. It defaults to 1 when unset.
	RSLSignerThreshold int

	// RefPatterns are the rule patterns the RSL signers are authorized for,
	// such as "git:refs/*".
	RefPatterns []string

	// Expires is the expiry recorded in the root and targets metadata.
	Expires time.Time

	// Version is the metadata version stamped on both the root and the
	// targets metadata. It defaults to 1. A successor to an applied policy
	// must not carry a lower version than the applied root.
	Version uint64
}

// AuthorPolicy signs a root of trust and a top-level targets rule file for p,
// writes them to store as a policy tree, and returns the tree ID. It sets no
// reference and records no RSL entry. The caller commits the tree onto the
// parent of its choosing and records the policy update.
//
// The authored state must pass the same verification the policy engine applies
// when loading it, so metadata that could not satisfy its own thresholds is
// refused before anything is written. That check covers the new state alone.
// A successor is only accepted on top of an applied policy whose root keys
// signed it, and every signer must be one of RootPublicKeys, so rotating to a
// disjoint set of root keys is not supported.
func AuthorPolicy(ctx context.Context, store gitstore.Storer, p AuthorPolicyParams) (githash.Hash, error) {
	if err := p.validate(); err != nil {
		return nil, err
	}

	// Thresholds count distinct identities. The same key passed twice is one
	// principal in the metadata, and SignEnvelope replaces a signature whose
	// key ID it already holds.
	rootSigners, err := signersFromCryptoSigners(p.RootSigners)
	if err != nil {
		return nil, err
	}

	rootPrincipals, err := principalsFromPublicKeys(p.RootPublicKeys)
	if err != nil {
		return nil, err
	}

	rslPrincipals, err := principalsFromPublicKeys(p.RSLSignerPublicKeys)
	if err != nil {
		return nil, err
	}

	for _, signer := range rootSigners {
		keyID, err := signer.KeyID()
		if err != nil {
			return nil, err
		}
		if !isKeyAuthorized(rootPrincipals, keyID) {
			return nil, fmt.Errorf("root signer %s is not among the root public keys", keyID)
		}
	}

	if len(rootPrincipals) < p.RootThreshold {
		return nil, fmt.Errorf("root threshold %d exceeds the %d distinct root public keys provided", p.RootThreshold, len(rootPrincipals))
	}
	if len(rootSigners) < p.RootThreshold {
		return nil, fmt.Errorf("root threshold %d exceeds the %d distinct root signers provided", p.RootThreshold, len(rootSigners))
	}
	if len(rslPrincipals) < p.rslSignerThreshold() {
		return nil, fmt.Errorf("RSL signer threshold %d exceeds the %d distinct RSL signer public keys provided", p.rslSignerThreshold(), len(rslPrincipals))
	}

	rootMetadata, err := p.rootMetadata(rootPrincipals)
	if err != nil {
		return nil, err
	}

	rootEnv, err := signMetadata(ctx, rootMetadata, rootSigners)
	if err != nil {
		return nil, err
	}

	targetsMetadata, err := p.targetsMetadata(rslPrincipals)
	if err != nil {
		return nil, err
	}

	targetsEnv, err := signMetadata(ctx, targetsMetadata, rootSigners)
	if err != nil {
		return nil, err
	}

	state := &policy.State{
		Metadata: &policy.StateMetadata{
			RootEnvelope:    rootEnv,
			TargetsEnvelope: targetsEnv,
		},
	}

	if err := state.Verify(ctx); err != nil {
		return nil, fmt.Errorf("authored policy does not verify: %w", err)
	}

	return state.WriteTree(store)
}

func (p AuthorPolicyParams) validate() error {
	if len(p.RootSigners) == 0 {
		return fmt.Errorf("at least one root signer must be provided")
	}
	for _, signer := range p.RootSigners {
		if signer == nil {
			return fmt.Errorf("root signer must not be nil")
		}
	}
	if len(p.RootPublicKeys) == 0 {
		return fmt.Errorf("at least one root public key must be provided")
	}
	if p.RootThreshold < 1 {
		return fmt.Errorf("root threshold must be at least 1")
	}
	if len(p.RSLSignerPublicKeys) == 0 {
		return fmt.Errorf("at least one RSL signer public key must be provided")
	}
	if p.RSLSignerThreshold < 0 {
		return fmt.Errorf("RSL signer threshold must not be negative")
	}
	if len(p.RefPatterns) == 0 {
		return fmt.Errorf("at least one ref pattern must be provided")
	}
	if p.Expires.IsZero() {
		return fmt.Errorf("expiry must be provided")
	}

	return nil
}

func (p AuthorPolicyParams) rslSignerThreshold() int {
	if p.RSLSignerThreshold == 0 {
		return 1
	}

	return p.RSLSignerThreshold
}

func (p AuthorPolicyParams) rootMetadata(rootPrincipals []tuf.Principal) (tuf.RootMetadata, error) {
	rootMetadata := tufv02.NewRootMetadata()
	rootMetadata.SetExpires(p.Expires.Format(time.RFC3339))
	if p.Version > rootMetadata.Version {
		rootMetadata.Version = p.Version
	}

	for _, principal := range rootPrincipals {
		if err := rootMetadata.AddRootPrincipal(principal); err != nil {
			return nil, err
		}
		if err := rootMetadata.AddPrimaryRuleFilePrincipal(principal); err != nil {
			return nil, err
		}
	}

	if err := rootMetadata.UpdateRootThreshold(p.RootThreshold); err != nil {
		return nil, err
	}
	if err := rootMetadata.UpdatePrimaryRuleFileThreshold(p.RootThreshold); err != nil {
		return nil, err
	}

	return rootMetadata, nil
}

func (p AuthorPolicyParams) targetsMetadata(rslPrincipals []tuf.Principal) (tuf.TargetsMetadata, error) {
	targetsMetadata := tufv02.NewTargetsMetadata()
	targetsMetadata.SetExpires(p.Expires.Format(time.RFC3339))
	if p.Version > targetsMetadata.Version {
		targetsMetadata.Version = p.Version
	}

	ids := make([]string, 0, len(rslPrincipals))
	for _, principal := range rslPrincipals {
		if err := targetsMetadata.AddPrincipal(principal); err != nil {
			return nil, err
		}
		ids = append(ids, principal.ID())
	}

	if err := targetsMetadata.AddRule(authorPolicyRuleName, ids, p.RefPatterns, p.rslSignerThreshold()); err != nil {
		return nil, err
	}

	return targetsMetadata, nil
}

func signMetadata(ctx context.Context, metadata any, signers []sslibdsse.SignerVerifier) (*sslibdsse.Envelope, error) {
	env, err := dsse.CreateEnvelope(metadata)
	if err != nil {
		return nil, err
	}

	for _, signer := range signers {
		env, err = dsse.SignEnvelope(ctx, env, signer)
		if err != nil {
			return nil, err
		}
	}

	return env, nil
}

// signersFromCryptoSigners returns the DSSE signers for signers in order,
// keeping the first of each distinct key ID.
func signersFromCryptoSigners(signers []crypto.Signer) ([]sslibdsse.SignerVerifier, error) {
	built := make([]sslibdsse.SignerVerifier, 0, len(signers))
	seen := map[string]bool{}
	for _, signer := range signers {
		dsseSigner, err := ssh.NewSignerFromCryptoSigner(signer)
		if err != nil {
			return nil, err
		}
		keyID, err := dsseSigner.KeyID()
		if err != nil {
			return nil, err
		}
		if seen[keyID] {
			continue
		}
		seen[keyID] = true
		built = append(built, dsseSigner)
	}

	return built, nil
}
