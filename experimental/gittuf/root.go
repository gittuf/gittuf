// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gittuf

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/gittuf/gittuf/experimental/gittuf/options/root"
	trustpolicyopts "github.com/gittuf/gittuf/experimental/gittuf/options/trustpolicy"
	"github.com/gittuf/gittuf/internal/dev"
	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/policy"
	policyopts "github.com/gittuf/gittuf/internal/policy/options/policy"
	"github.com/gittuf/gittuf/internal/signerverifier/common"
	"github.com/gittuf/gittuf/internal/signerverifier/dsse"
	"github.com/gittuf/gittuf/internal/signerverifier/sigstore"
	"github.com/gittuf/gittuf/internal/signerverifier/ssh"
	sslibdsse "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
	"github.com/gittuf/gittuf/internal/tuf"
	tufv01 "github.com/gittuf/gittuf/internal/tuf/v01"
	tufv02 "github.com/gittuf/gittuf/internal/tuf/v02"
	"github.com/secure-systems-lab/go-securesystemslib/signerverifier"
)

var (
	ErrNoHookName = errors.New("hook name not provided")
)

// InitializeRoot is the interface for the user to create the repository's root
// of trust.
func (r *Repository) InitializeRoot(ctx context.Context, signer sslibdsse.SignerVerifier, signCommit bool, opts ...root.Option) error {
	if signCommit {
		slog.Debug("Checking if Git signing is configured...")
		err := r.r.CanSign()
		if err != nil {
			return err
		}
	}

	options := &root.Options{}
	for _, fn := range opts {
		fn(options)
	}

	var (
		publicKeyRaw *signerverifier.SSLibKey
		err          error
	)

	switch signer := signer.(type) {
	case *ssh.Signer:
		publicKeyRaw = signer.MetadataKey()
	case *sigstore.Signer:
		publicKeyRaw, err = signer.MetadataKey()
		if err != nil {
			return err
		}
	default:
		return common.ErrUnknownKeyType
	}
	publicKey := tufv01.NewKeyFromSSLibKey(publicKeyRaw)

	slog.Debug("Creating initial root metadata...")
	rootMetadata, err := policy.InitializeRootMetadata(publicKey)
	if err != nil {
		return err
	}

	if options.RepositoryLocation != "" {
		slog.Debug("Setting repository location...")
		rootMetadata.SetRepositoryLocation(options.RepositoryLocation)
	}

	env, err := dsse.CreateEnvelope(rootMetadata)
	if err != nil {
		return err
	}

	slog.Debug(fmt.Sprintf("Signing initial root metadata using '%s'...", publicKey.KeyID))
	env, err = dsse.SignEnvelope(ctx, env, signer)
	if err != nil {
		return err
	}

	state := &policy.State{
		Metadata: &policy.StateMetadata{
			RootEnvelope: env,
		},
	}

	commitMessage := "Initialize root of trust"

	slog.Debug("Committing policy...")
	return state.Commit(r.r, commitMessage, options.CreateRSLEntry, signCommit)
}

func (r *Repository) SetRepositoryLocation(ctx context.Context, signer sslibdsse.SignerVerifier, location string, signCommit bool, opts ...trustpolicyopts.Option) error {
	if signCommit {
		slog.Debug("Checking if Git signing is configured...")
		err := r.r.CanSign()
		if err != nil {
			return err
		}
	}

	options := &trustpolicyopts.Options{}
	for _, fn := range opts {
		fn(options)
	}

	rootKeyID, err := signer.KeyID()
	if err != nil {
		return err
	}

	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r, policy.PolicyStagingRef, policyopts.BypassRSL())
	if err != nil {
		return err
	}

	rootMetadata, err := r.loadRootMetadata(state, rootKeyID)
	if err != nil {
		return err
	}

	rootMetadata.SetRepositoryLocation(location)

	commitMessage := fmt.Sprintf("Set repository location to '%s' in root", location)
	return r.updateRootMetadata(ctx, state, signer, rootMetadata, commitMessage, options.CreateRSLEntry, signCommit)
}

// AddRootKey is the interface for the user to add an authorized key
// for the Root role.
func (r *Repository) AddRootKey(ctx context.Context, signer sslibdsse.SignerVerifier, newRootKey tuf.Principal, signCommit bool, opts ...trustpolicyopts.Option) error {
	if signCommit {
		slog.Debug("Checking if Git signing is configured...")
		err := r.r.CanSign()
		if err != nil {
			return err
		}
	}

	options := &trustpolicyopts.Options{}
	for _, fn := range opts {
		fn(options)
	}

	rootKeyID, err := signer.KeyID()
	if err != nil {
		return err
	}

	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r, policy.PolicyStagingRef, policyopts.BypassRSL())
	if err != nil {
		return err
	}

	rootMetadata, err := r.loadRootMetadata(state, rootKeyID)
	if err != nil {
		return err
	}

	slog.Debug("Adding root key...")
	if err := rootMetadata.AddRootPrincipal(newRootKey); err != nil {
		return err
	}

	commitMessage := fmt.Sprintf("Add root key '%s' to root", newRootKey.ID())
	return r.updateRootMetadata(ctx, state, signer, rootMetadata, commitMessage, options.CreateRSLEntry, signCommit)
}

// RemoveRootKey is the interface for the user to de-authorize a key
// trusted to sign the Root role.
func (r *Repository) RemoveRootKey(ctx context.Context, signer sslibdsse.SignerVerifier, keyID string, signCommit bool, opts ...trustpolicyopts.Option) error {
	if signCommit {
		slog.Debug("Checking if Git signing is configured...")
		err := r.r.CanSign()
		if err != nil {
			return err
		}
	}

	options := &trustpolicyopts.Options{}
	for _, fn := range opts {
		fn(options)
	}

	rootKeyID, err := signer.KeyID()
	if err != nil {
		return err
	}

	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r, policy.PolicyStagingRef, policyopts.BypassRSL())
	if err != nil {
		return err
	}

	rootMetadata, err := r.loadRootMetadata(state, rootKeyID)
	if err != nil {
		return err
	}

	slog.Debug("Removing root key...")
	if err := rootMetadata.DeleteRootPrincipal(keyID); err != nil {
		return err
	}

	commitMessage := fmt.Sprintf("Remove root key '%s' from root", keyID)
	return r.updateRootMetadata(ctx, state, signer, rootMetadata, commitMessage, options.CreateRSLEntry, signCommit)
}

// AddTopLevelTargetsKey is the interface for the user to add an authorized key
// for the top level Targets role / policy file.
func (r *Repository) AddTopLevelTargetsKey(ctx context.Context, signer sslibdsse.SignerVerifier, targetsKey tuf.Principal, signCommit bool, opts ...trustpolicyopts.Option) error {
	if signCommit {
		slog.Debug("Checking if Git signing is configured...")
		err := r.r.CanSign()
		if err != nil {
			return err
		}
	}

	options := &trustpolicyopts.Options{}
	for _, fn := range opts {
		fn(options)
	}

	rootKeyID, err := signer.KeyID()
	if err != nil {
		return err
	}

	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r, policy.PolicyStagingRef, policyopts.BypassRSL())
	if err != nil {
		return err
	}

	rootMetadata, err := r.loadRootMetadata(state, rootKeyID)
	if err != nil {
		return err
	}

	slog.Debug("Adding policy key...")
	if err := rootMetadata.AddPrimaryRuleFilePrincipal(targetsKey); err != nil {
		return fmt.Errorf("failed to add policy key: %w", err)
	}

	commitMessage := fmt.Sprintf("Add policy key '%s' to root", targetsKey.ID())
	return r.updateRootMetadata(ctx, state, signer, rootMetadata, commitMessage, options.CreateRSLEntry, signCommit)
}

// RemoveTopLevelTargetsKey is the interface for the user to de-authorize a key
// trusted to sign the top level Targets role / policy file.
func (r *Repository) RemoveTopLevelTargetsKey(ctx context.Context, signer sslibdsse.SignerVerifier, targetsKeyID string, signCommit bool, opts ...trustpolicyopts.Option) error {
	if signCommit {
		slog.Debug("Checking if Git signing is configured...")
		err := r.r.CanSign()
		if err != nil {
			return err
		}
	}

	options := &trustpolicyopts.Options{}
	for _, fn := range opts {
		fn(options)
	}

	rootKeyID, err := signer.KeyID()
	if err != nil {
		return err
	}

	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r, policy.PolicyStagingRef, policyopts.BypassRSL())
	if err != nil {
		return err
	}

	rootMetadata, err := r.loadRootMetadata(state, rootKeyID)
	if err != nil {
		return err
	}

	slog.Debug("Removing policy key...")
	if err := rootMetadata.DeletePrimaryRuleFilePrincipal(targetsKeyID); err != nil {
		return err
	}

	commitMessage := fmt.Sprintf("Remove policy key '%s' from root", targetsKeyID)
	return r.updateRootMetadata(ctx, state, signer, rootMetadata, commitMessage, options.CreateRSLEntry, signCommit)
}

// AddGitHubApp is the interface for the user to add the authorized key for the
// trusted GitHub app. This key is used to verify GitHub pull request approval
// attestation signatures recorded by the app.
func (r *Repository) AddGitHubApp(ctx context.Context, signer sslibdsse.SignerVerifier, appKey tuf.Principal, signCommit bool, opts ...trustpolicyopts.Option) error {
	if signCommit {
		slog.Debug("Checking if Git signing is configured...")
		err := r.r.CanSign()
		if err != nil {
			return err
		}
	}

	options := &trustpolicyopts.Options{}
	for _, fn := range opts {
		fn(options)
	}

	rootKeyID, err := signer.KeyID()
	if err != nil {
		return err
	}

	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r, policy.PolicyStagingRef, policyopts.BypassRSL())
	if err != nil {
		return err
	}

	rootMetadata, err := r.loadRootMetadata(state, rootKeyID)
	if err != nil {
		return err
	}

	slog.Debug("Adding GitHub app key...")
	if err := rootMetadata.AddGitHubAppPrincipal(tuf.GitHubAppRoleName, appKey); err != nil {
		return fmt.Errorf("failed to add GitHub app key: %w", err)
	}

	commitMessage := fmt.Sprintf("Add GitHub app key '%s' to root", appKey.ID())
	return r.updateRootMetadata(ctx, state, signer, rootMetadata, commitMessage, options.CreateRSLEntry, signCommit)
}

// RemoveGitHubApp is the interface for the user to de-authorize the key for the
// special GitHub app role.
func (r *Repository) RemoveGitHubApp(ctx context.Context, signer sslibdsse.SignerVerifier, signCommit bool, opts ...trustpolicyopts.Option) error {
	if signCommit {
		slog.Debug("Checking if Git signing is configured...")
		err := r.r.CanSign()
		if err != nil {
			return err
		}
	}

	options := &trustpolicyopts.Options{}
	for _, fn := range opts {
		fn(options)
	}

	rootKeyID, err := signer.KeyID()
	if err != nil {
		return err
	}

	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r, policy.PolicyStagingRef, policyopts.BypassRSL())
	if err != nil {
		return err
	}

	rootMetadata, err := r.loadRootMetadata(state, rootKeyID)
	if err != nil {
		return err
	}

	slog.Debug("Removing GitHub app key...")
	rootMetadata.DeleteGitHubAppPrincipal(tuf.GitHubAppRoleName)

	commitMessage := "Remove GitHub app key from root"
	return r.updateRootMetadata(ctx, state, signer, rootMetadata, commitMessage, options.CreateRSLEntry, signCommit)
}

// TrustGitHubApp updates the root metadata to mark GitHub app pull request
// approvals as trusted.
func (r *Repository) TrustGitHubApp(ctx context.Context, signer sslibdsse.SignerVerifier, signCommit bool, opts ...trustpolicyopts.Option) error {
	if signCommit {
		slog.Debug("Checking if Git signing is configured...")
		err := r.r.CanSign()
		if err != nil {
			return err
		}
	}

	options := &trustpolicyopts.Options{}
	for _, fn := range opts {
		fn(options)
	}

	rootKeyID, err := signer.KeyID()
	if err != nil {
		return err
	}

	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r, policy.PolicyStagingRef, policyopts.BypassRSL())
	if err != nil {
		return err
	}

	rootMetadata, err := r.loadRootMetadata(state, rootKeyID)
	if err != nil {
		return err
	}

	if rootMetadata.IsGitHubAppApprovalTrusted() {
		slog.Debug("GitHub app approvals are already trusted, exiting...")
		return nil
	}

	slog.Debug("Marking GitHub app approvals as trusted in root...")
	rootMetadata.EnableGitHubAppApprovals()

	commitMessage := "Mark GitHub app approvals as trusted"
	return r.updateRootMetadata(ctx, state, signer, rootMetadata, commitMessage, options.CreateRSLEntry, signCommit)
}

// UntrustGitHubApp updates the root metadata to mark GitHub app pull request
// approvals as untrusted.
func (r *Repository) UntrustGitHubApp(ctx context.Context, signer sslibdsse.SignerVerifier, signCommit bool, opts ...trustpolicyopts.Option) error {
	if signCommit {
		slog.Debug("Checking if Git signing is configured...")
		err := r.r.CanSign()
		if err != nil {
			return err
		}
	}

	options := &trustpolicyopts.Options{}
	for _, fn := range opts {
		fn(options)
	}

	rootKeyID, err := signer.KeyID()
	if err != nil {
		return err
	}

	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r, policy.PolicyStagingRef, policyopts.BypassRSL())
	if err != nil {
		return err
	}

	rootMetadata, err := r.loadRootMetadata(state, rootKeyID)
	if err != nil {
		return err
	}

	if !rootMetadata.IsGitHubAppApprovalTrusted() {
		slog.Debug("GitHub app approvals are already untrusted, exiting...")
		return nil
	}

	slog.Debug("Marking GitHub app approvals as untrusted in root...")
	rootMetadata.DisableGitHubAppApprovals()

	commitMessage := "Mark GitHub app approvals as untrusted"
	return r.updateRootMetadata(ctx, state, signer, rootMetadata, commitMessage, options.CreateRSLEntry, signCommit)
}

// UpdateRootThreshold sets the threshold of valid signatures required for the
// Root role.
func (r *Repository) UpdateRootThreshold(ctx context.Context, signer sslibdsse.SignerVerifier, threshold int, signCommit bool, opts ...trustpolicyopts.Option) error {
	if signCommit {
		slog.Debug("Checking if Git signing is configured...")
		err := r.r.CanSign()
		if err != nil {
			return err
		}
	}

	options := &trustpolicyopts.Options{}
	for _, fn := range opts {
		fn(options)
	}

	rootKeyID, err := signer.KeyID()
	if err != nil {
		return err
	}

	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r, policy.PolicyStagingRef, policyopts.BypassRSL())
	if err != nil {
		return err
	}

	rootMetadata, err := r.loadRootMetadata(state, rootKeyID)
	if err != nil {
		return err
	}

	slog.Debug("Updating root threshold...")
	if err := rootMetadata.UpdateRootThreshold(threshold); err != nil {
		return err
	}

	commitMessage := fmt.Sprintf("Update root threshold to %d", threshold)
	return r.updateRootMetadata(ctx, state, signer, rootMetadata, commitMessage, options.CreateRSLEntry, signCommit)
}

// UpdateTopLevelTargetsThreshold sets the threshold of valid signatures
// required for the top level Targets role.
func (r *Repository) UpdateTopLevelTargetsThreshold(ctx context.Context, signer sslibdsse.SignerVerifier, threshold int, signCommit bool, opts ...trustpolicyopts.Option) error {
	if signCommit {
		slog.Debug("Checking if Git signing is configured...")
		err := r.r.CanSign()
		if err != nil {
			return err
		}
	}

	options := &trustpolicyopts.Options{}
	for _, fn := range opts {
		fn(options)
	}

	rootKeyID, err := signer.KeyID()
	if err != nil {
		return err
	}

	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r, policy.PolicyStagingRef, policyopts.BypassRSL())
	if err != nil {
		return err
	}

	rootMetadata, err := r.loadRootMetadata(state, rootKeyID)
	if err != nil {
		return err
	}

	slog.Debug("Updating policy threshold...")
	if err := rootMetadata.UpdatePrimaryRuleFileThreshold(threshold); err != nil {
		return err
	}

	commitMessage := fmt.Sprintf("Update policy threshold to %d", threshold)
	return r.updateRootMetadata(ctx, state, signer, rootMetadata, commitMessage, options.CreateRSLEntry, signCommit)
}

// AddGlobalRuleThreshold adds a threshold global rule to the root metadata.
func (r *Repository) AddGlobalRuleThreshold(ctx context.Context, signer sslibdsse.SignerVerifier, name string, patterns []string, threshold int, signCommit bool, opts ...trustpolicyopts.Option) error {
	if !dev.InDevMode() {
		return dev.ErrNotInDevMode
	}

	options := &trustpolicyopts.Options{}
	for _, fn := range opts {
		fn(options)
	}

	if signCommit {
		slog.Debug("Checking if Git signing is configured...")
		err := r.r.CanSign()
		if err != nil {
			return err
		}
	}

	rootKeyID, err := signer.KeyID()
	if err != nil {
		return err
	}

	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r, policy.PolicyStagingRef, policyopts.BypassRSL())
	if err != nil {
		return err
	}

	rootMetadata, err := r.loadRootMetadata(state, rootKeyID)
	if err != nil {
		return err
	}

	slog.Debug("Adding threshold global rule...")
	if err := rootMetadata.AddGlobalRule(tufv01.NewGlobalRuleThreshold(name, patterns, threshold)); err != nil {
		return err
	}

	commitMessage := fmt.Sprintf("Add global rule (%s) '%s' to root metadata", tuf.GlobalRuleThresholdType, name)
	return r.updateRootMetadata(ctx, state, signer, rootMetadata, commitMessage, options.CreateRSLEntry, signCommit)
}

// AddGlobalRuleBlockForcePushes adds a global rule that blocks force pushes to the root metadata.
func (r *Repository) AddGlobalRuleBlockForcePushes(ctx context.Context, signer sslibdsse.SignerVerifier, name string, patterns []string, signCommit bool, opts ...trustpolicyopts.Option) error {
	if !dev.InDevMode() {
		return dev.ErrNotInDevMode
	}

	options := &trustpolicyopts.Options{}
	for _, fn := range opts {
		fn(options)
	}

	if signCommit {
		slog.Debug("Checking if Git signing is configured...")
		err := r.r.CanSign()
		if err != nil {
			return err
		}
	}

	rootKeyID, err := signer.KeyID()
	if err != nil {
		return err
	}

	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r, policy.PolicyStagingRef, policyopts.BypassRSL())
	if err != nil {
		return err
	}

	rootMetadata, err := r.loadRootMetadata(state, rootKeyID)
	if err != nil {
		return err
	}

	globalRule, err := tufv01.NewGlobalRuleBlockForcePushes(name, patterns)
	if err != nil {
		return err
	}

	slog.Debug("Adding threshold global rule...")
	if err := rootMetadata.AddGlobalRule(globalRule); err != nil {
		return err
	}

	commitMessage := fmt.Sprintf("Add global rule (%s) '%s' to root metadata", tuf.GlobalRuleBlockForcePushesType, name)
	return r.updateRootMetadata(ctx, state, signer, rootMetadata, commitMessage, options.CreateRSLEntry, signCommit)
}

// UpdateGlobalRuleThreshold updates an existing threshold global rule in the root metadata.
func (r *Repository) UpdateGlobalRuleThreshold(ctx context.Context, signer sslibdsse.SignerVerifier, name string, patterns []string, threshold int, signCommit bool, opts ...trustpolicyopts.Option) error {
	if !dev.InDevMode() {
		return dev.ErrNotInDevMode
	}

	if signCommit {
		slog.Debug("Checking if Git signing is configured...")
		err := r.r.CanSign()
		if err != nil {
			return err
		}
	}

	options := &trustpolicyopts.Options{}
	for _, fn := range opts {
		fn(options)
	}

	rootKeyID, err := signer.KeyID()
	if err != nil {
		return err
	}

	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r, policy.PolicyStagingRef)
	if err != nil {
		return err
	}

	rootMetadata, err := r.loadRootMetadata(state, rootKeyID)
	if err != nil {
		return err
	}

	slog.Debug("Updating threshold global rule...")
	if err := rootMetadata.UpdateGlobalRule(tufv01.NewGlobalRuleThreshold(name, patterns, threshold)); err != nil {
		return err
	}

	commitMessage := fmt.Sprintf("Update global rule '%s' in root metadata", name)
	return r.updateRootMetadata(ctx, state, signer, rootMetadata, commitMessage, options.CreateRSLEntry, signCommit)
}

// UpdateGlobalRuleBlockForcePushes updates an existing block-force-pushes global rule in the root metadata.
func (r *Repository) UpdateGlobalRuleBlockForcePushes(ctx context.Context, signer sslibdsse.SignerVerifier, name string, patterns []string, signCommit bool, opts ...trustpolicyopts.Option) error {
	if !dev.InDevMode() {
		return dev.ErrNotInDevMode
	}

	if signCommit {
		slog.Debug("Checking if Git signing is configured...")
		err := r.r.CanSign()
		if err != nil {
			return err
		}
	}

	options := &trustpolicyopts.Options{}
	for _, fn := range opts {
		fn(options)
	}

	rootKeyID, err := signer.KeyID()
	if err != nil {
		return err
	}

	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r, policy.PolicyStagingRef)
	if err != nil {
		return err
	}

	rootMetadata, err := r.loadRootMetadata(state, rootKeyID)
	if err != nil {
		return err
	}

	globalRule, err := tufv01.NewGlobalRuleBlockForcePushes(name, patterns)
	if err != nil {
		return err
	}

	slog.Debug("Updating block-force-pushes global rule...")
	if err := rootMetadata.UpdateGlobalRule(globalRule); err != nil {
		return err
	}

	commitMessage := fmt.Sprintf("Update global rule '%s' in root metadata", name)
	return r.updateRootMetadata(ctx, state, signer, rootMetadata, commitMessage, options.CreateRSLEntry, signCommit)
}

// RemoveGlobalRule removes a global rule from the root metadata.
func (r *Repository) RemoveGlobalRule(ctx context.Context, signer sslibdsse.SignerVerifier, name string, signCommit bool, opts ...trustpolicyopts.Option) error {
	if !dev.InDevMode() {
		return dev.ErrNotInDevMode
	}

	if signCommit {
		slog.Debug("Checking if Git signing is configured...")
		err := r.r.CanSign()
		if err != nil {
			return err
		}
	}

	options := &trustpolicyopts.Options{}
	for _, fn := range opts {
		fn(options)
	}

	rootKeyID, err := signer.KeyID()
	if err != nil {
		return err
	}

	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r, policy.PolicyStagingRef, policyopts.BypassRSL())
	if err != nil {
		return err
	}

	rootMetadata, err := r.loadRootMetadata(state, rootKeyID)
	if err != nil {
		return err
	}

	slog.Debug("Removing global rule...")
	if err := rootMetadata.DeleteGlobalRule(name); err != nil {
		return err
	}

	commitMessage := fmt.Sprintf("Remove global rule '%s' from root metadata", name)
	return r.updateRootMetadata(ctx, state, signer, rootMetadata, commitMessage, options.CreateRSLEntry, signCommit)
}

func (r *Repository) AddPropagationDirective(ctx context.Context, signer sslibdsse.SignerVerifier, directiveName, upstreamRepository, upstreamReference, downstreamReference, downstreamPath string, signCommit bool, opts ...trustpolicyopts.Option) error {
	if !dev.InDevMode() {
		return dev.ErrNotInDevMode
	}

	if signCommit {
		slog.Debug("Checking if Git signing is configured...")
		err := r.r.CanSign()
		if err != nil {
			return err
		}
	}

	options := &trustpolicyopts.Options{}
	for _, fn := range opts {
		fn(options)
	}

	rootKeyID, err := signer.KeyID()
	if err != nil {
		return err
	}

	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r, policy.PolicyStagingRef, policyopts.BypassRSL())
	if err != nil {
		return err
	}

	rootMetadata, err := r.loadRootMetadata(state, rootKeyID)
	if err != nil {
		return err
	}

	slog.Debug("Adding propagation directive...")
	var directive tuf.PropagationDirective
	switch rootMetadata.(type) {
	case *tufv01.RootMetadata:
		directive = tufv01.NewPropagationDirective(directiveName, upstreamRepository, upstreamReference, downstreamReference, downstreamPath)
	case *tufv02.RootMetadata:
		directive = tufv02.NewPropagationDirective(directiveName, upstreamRepository, upstreamReference, downstreamReference, downstreamPath)
	}

	if err := rootMetadata.AddPropagationDirective(directive); err != nil {
		return err
	}

	commitMessage := fmt.Sprintf("Add propagation directive '%s' to root metadata", directiveName)
	return r.updateRootMetadata(ctx, state, signer, rootMetadata, commitMessage, options.CreateRSLEntry, signCommit)
}

func (r *Repository) RemovePropagationDirective(ctx context.Context, signer sslibdsse.SignerVerifier, name string, signCommit bool, opts ...trustpolicyopts.Option) error {
	if !dev.InDevMode() {
		return dev.ErrNotInDevMode
	}

	if signCommit {
		slog.Debug("Checking if Git signing is configured...")
		err := r.r.CanSign()
		if err != nil {
			return err
		}
	}

	options := &trustpolicyopts.Options{}
	for _, fn := range opts {
		fn(options)
	}

	rootKeyID, err := signer.KeyID()
	if err != nil {
		return err
	}

	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r, policy.PolicyStagingRef, policyopts.BypassRSL())
	if err != nil {
		return err
	}

	rootMetadata, err := r.loadRootMetadata(state, rootKeyID)
	if err != nil {
		return err
	}

	slog.Debug("Removing propagation directive...")
	if err := rootMetadata.DeletePropagationDirective(name); err != nil {
		return err
	}

	commitMessage := fmt.Sprintf("Remove propagation directive '%s' from root metadata", name)
	return r.updateRootMetadata(ctx, state, signer, rootMetadata, commitMessage, options.CreateRSLEntry, signCommit)
}

// AddHook defines the workflow for adding a file to be executed as a hook. It
// writes the hook file, populates all fields in the hooks metadata associated
// with this file and commits it to the root of trust metadata.
func (r *Repository) AddHook(ctx context.Context, signer sslibdsse.SignerVerifier, stages []tuf.HookStage, hookName string, hookBytes []byte, environment tuf.HookEnvironment, modules, principalIDs []string, signCommit bool, opts ...trustpolicyopts.Option) error {
	if !dev.InDevMode() {
		return dev.ErrNotInDevMode
	}

	if signCommit {
		slog.Debug("Checking if Git signing is configured...")
		err := r.r.CanSign()
		if err != nil {
			return err
		}
	}

	if hookName == "" {
		return ErrNoHookName
	}

	options := &trustpolicyopts.Options{}
	for _, fn := range opts {
		fn(options)
	}

	rootKeyID, err := signer.KeyID()
	if err != nil {
		return err
	}

	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r, policy.PolicyStagingRef, policyopts.BypassRSL())
	if err != nil {
		return err
	}

	rootMetadata, err := r.loadRootMetadata(state, rootKeyID)
	if err != nil {
		return err
	}

	var hashes = make(map[string]string, 2)
	blobID, err := r.r.WriteBlob(hookBytes)
	if err != nil {
		return err
	}
	// TODO: hash agility
	hashes[gitinterface.GitBlobHashName] = blobID.String()

	sha256Hash := sha256.New()
	sha256Hash.Write(hookBytes)
	hashes[gitinterface.SHA256HashName] = hex.EncodeToString(sha256Hash.Sum(nil))

	slog.Debug("Adding hook to rule file...")
	hook, err := rootMetadata.AddHook(stages, hookName, principalIDs, hashes, environment, modules)
	if err != nil {
		return err
	}

	for _, stage := range stages {
		state.Hooks[stage] = append(state.Hooks[stage], hook)
	}

	commitMessage := fmt.Sprintf("Add hook '%s' to root metadata", hookName)
	return r.updateRootMetadata(ctx, state, signer, rootMetadata, commitMessage, options.CreateRSLEntry, signCommit)
}

// RemoveHook defines the workflow for removing a hook defined in gittuf policy.
func (r *Repository) RemoveHook(ctx context.Context, signer sslibdsse.SignerVerifier, stages []tuf.HookStage, hookName string, signCommit bool, opts ...trustpolicyopts.Option) error {
	if !dev.InDevMode() {
		return dev.ErrNotInDevMode
	}

	if signCommit {
		slog.Debug("Checking if Git signing is configured...")
		err := r.r.CanSign()
		if err != nil {
			return err
		}
	}

	options := &trustpolicyopts.Options{}
	for _, fn := range opts {
		fn(options)
	}

	rootKeyID, err := signer.KeyID()
	if err != nil {
		return err
	}

	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r, policy.PolicyStagingRef, policyopts.BypassRSL())
	if err != nil {
		return err
	}

	rootMetadata, err := r.loadRootMetadata(state, rootKeyID)
	if err != nil {
		return err
	}

	slog.Debug("Removing hook...")
	err = rootMetadata.RemoveHook(stages, hookName)
	if err != nil {
		return err
	}

	for _, stage := range stages {
		updatedHooks, err := rootMetadata.GetHooks(stage)
		if err != nil {
			return err
		}

		state.Hooks[stage] = updatedHooks
	}

	commitMessage := fmt.Sprintf("Remove hook '%s' from root metadata", hookName)
	return r.updateRootMetadata(ctx, state, signer, rootMetadata, commitMessage, options.CreateRSLEntry, signCommit)
}

// EnableController makes the current repository a "controller" repository used
// to specify gittuf policies for other repositories.
func (r *Repository) EnableController(ctx context.Context, signer sslibdsse.SignerVerifier, signCommit bool, opts ...trustpolicyopts.Option) error {
	if !dev.InDevMode() {
		return dev.ErrNotInDevMode
	}

	if signCommit {
		slog.Debug("Checking if Git signing is configured...")
		err := r.r.CanSign()
		if err != nil {
			return err
		}
	}

	options := &trustpolicyopts.Options{}
	for _, fn := range opts {
		fn(options)
	}

	rootKeyID, err := signer.KeyID()
	if err != nil {
		return err
	}

	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r, policy.PolicyStagingRef, policyopts.BypassRSL())
	if err != nil {
		return err
	}

	rootMetadata, err := r.loadRootMetadata(state, rootKeyID)
	if err != nil {
		return err
	}

	slog.Debug("Making repository controller...")
	if err := rootMetadata.EnableController(); err != nil {
		return err
	}

	commitMessage := "Make repository controller"
	return r.updateRootMetadata(ctx, state, signer, rootMetadata, commitMessage, options.CreateRSLEntry, signCommit)
}

// DisableController makes the repository not a controller for a network of
// gittuf repositories. Any policies declared in this repository will not be
// enforced for other repositories part of the network.
func (r *Repository) DisableController(ctx context.Context, signer sslibdsse.SignerVerifier, signCommit bool, opts ...trustpolicyopts.Option) error {
	if !dev.InDevMode() {
		return dev.ErrNotInDevMode
	}

	if signCommit {
		slog.Debug("Checking if Git signing is configured...")
		err := r.r.CanSign()
		if err != nil {
			return err
		}
	}

	options := &trustpolicyopts.Options{}
	for _, fn := range opts {
		fn(options)
	}

	rootKeyID, err := signer.KeyID()
	if err != nil {
		return err
	}

	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r, policy.PolicyStagingRef, policyopts.BypassRSL())
	if err != nil {
		return err
	}

	rootMetadata, err := r.loadRootMetadata(state, rootKeyID)
	if err != nil {
		return err
	}

	slog.Debug("Disabling repository as controller...")
	if err := rootMetadata.DisableController(); err != nil {
		return err
	}

	commitMessage := "Disable repository as controller"
	return r.updateRootMetadata(ctx, state, signer, rootMetadata, commitMessage, options.CreateRSLEntry, signCommit)
}

// AddControllerRepository adds a repository as a controller to the current
// repository.
func (r *Repository) AddControllerRepository(ctx context.Context, signer sslibdsse.SignerVerifier, repositoryName, repositoryLocation string, initialRootPrincipals []tuf.Principal, signCommit bool, opts ...trustpolicyopts.Option) error {
	if !dev.InDevMode() {
		return dev.ErrNotInDevMode
	}

	if signCommit {
		slog.Debug("Checking if Git signing is configured...")
		err := r.r.CanSign()
		if err != nil {
			return err
		}
	}

	options := &trustpolicyopts.Options{}
	for _, fn := range opts {
		fn(options)
	}

	rootKeyID, err := signer.KeyID()
	if err != nil {
		return err
	}

	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r, policy.PolicyStagingRef, policyopts.BypassRSL())
	if err != nil {
		return err
	}

	rootMetadata, err := r.loadRootMetadata(state, rootKeyID)
	if err != nil {
		return err
	}

	slog.Debug("Adding controller repository...")
	if err := rootMetadata.AddControllerRepository(repositoryName, repositoryLocation, initialRootPrincipals); err != nil {
		return err
	}

	commitMessage := "Add controller repository"
	return r.updateRootMetadata(ctx, state, signer, rootMetadata, commitMessage, options.CreateRSLEntry, signCommit)
}

// AddNetworkRepository adds a repository as part of the network overseen by the
// current repository.
func (r *Repository) AddNetworkRepository(ctx context.Context, signer sslibdsse.SignerVerifier, repositoryName, repositoryLocation string, initialRootPrincipals []tuf.Principal, signCommit bool, opts ...trustpolicyopts.Option) error {
	if !dev.InDevMode() {
		return dev.ErrNotInDevMode
	}

	if signCommit {
		slog.Debug("Checking if Git signing is configured...")
		err := r.r.CanSign()
		if err != nil {
			return err
		}
	}

	options := &trustpolicyopts.Options{}
	for _, fn := range opts {
		fn(options)
	}

	rootKeyID, err := signer.KeyID()
	if err != nil {
		return err
	}

	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r, policy.PolicyStagingRef, policyopts.BypassRSL())
	if err != nil {
		return err
	}

	rootMetadata, err := r.loadRootMetadata(state, rootKeyID)
	if err != nil {
		return err
	}

	slog.Debug("Adding network repository...")
	if err := rootMetadata.AddNetworkRepository(repositoryName, repositoryLocation, initialRootPrincipals); err != nil {
		return err
	}

	commitMessage := "Add network repository"
	return r.updateRootMetadata(ctx, state, signer, rootMetadata, commitMessage, options.CreateRSLEntry, signCommit)
}

// SignRoot adds a signature to the Root envelope. Note that the metadata itself
// is not modified, so its version remains the same.
func (r *Repository) SignRoot(ctx context.Context, signer sslibdsse.SignerVerifier, signCommit bool, opts ...trustpolicyopts.Option) error {
	if signCommit {
		slog.Debug("Checking if Git signing is configured...")
		err := r.r.CanSign()
		if err != nil {
			return err
		}
	}

	options := &trustpolicyopts.Options{}
	for _, fn := range opts {
		fn(options)
	}

	keyID, err := signer.KeyID()
	if err != nil {
		return err
	}

	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r, policy.PolicyStagingRef, policyopts.BypassRSL())
	if err != nil {
		return err
	}

	env := state.Metadata.RootEnvelope

	slog.Debug(fmt.Sprintf("Signing root metadata using '%s'...", keyID))
	env, err = dsse.SignEnvelope(ctx, env, signer)
	if err != nil {
		return err
	}

	state.Metadata.RootEnvelope = env

	commitMessage := fmt.Sprintf("Add signature from key '%s' to root metadata", keyID)

	slog.Debug("Committing policy...")
	return state.Commit(r.r, commitMessage, options.CreateRSLEntry, signCommit)
}

func (r *Repository) loadRootMetadata(state *policy.State, keyID string) (tuf.RootMetadata, error) {
	slog.Debug("Loading current root metadata...")
	rootMetadata, err := state.GetRootMetadata(false)
	if err != nil {
		return nil, err
	}

	authorizedPrincipals, err := rootMetadata.GetRootPrincipals()
	if err != nil {
		return nil, err
	}

	if !isKeyAuthorized(authorizedPrincipals, keyID) {
		return nil, ErrUnauthorizedKey
	}

	return rootMetadata, nil
}

func (r *Repository) updateRootMetadata(ctx context.Context, state *policy.State, signer sslibdsse.SignerVerifier, rootMetadata tuf.RootMetadata, commitMessage string, createRSLEntry, signCommit bool) error {
	rootMetadataBytes, err := json.Marshal(rootMetadata)
	if err != nil {
		return err
	}

	env := state.Metadata.RootEnvelope
	env.Signatures = []sslibdsse.Signature{}
	env.Payload = base64.StdEncoding.EncodeToString(rootMetadataBytes)

	slog.Debug("Signing updated root metadata...")
	env, err = dsse.SignEnvelope(ctx, env, signer)
	if err != nil {
		return err
	}

	state.Metadata.RootEnvelope = env

	slog.Debug("Committing policy...")
	return state.Commit(r.r, commitMessage, createRSLEntry, signCommit)
}

// ListGlobalRules returns a list of all global rules as an array of tuf.GlobalRules.
func (r *Repository) ListGlobalRules(ctx context.Context) ([]tuf.GlobalRule, error) {
	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r, policy.PolicyStagingRef)
	if err != nil {
		return nil, err
	}

	rootMetadata, err := state.GetRootMetadata(false)
	if err != nil {
		return nil, err
	}

	return rootMetadata.GetGlobalRules(), nil
}
