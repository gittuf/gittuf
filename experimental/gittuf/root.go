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
	ErrNoHookName         = errors.New("hook name not provided")
	ErrInvalidHookTimeout = errors.New("hook timeout must be greater than 1 second")
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

	if _, err := r.r.GetReference(policy.PolicyRef); err == nil {
		return ErrCannotReinitialize
	} else if !errors.Is(err, gitinterface.ErrReferenceNotFound) {
		return err
	}

	if _, err := r.r.GetReference(policy.PolicyStagingRef); err == nil {
		state, err := policy.LoadCurrentState(ctx, r.r, policy.PolicyStagingRef, policyopts.BypassRSL())
		if err != nil {
			return err
		}
		if state != nil && state.Metadata != nil && state.Metadata.RootEnvelope != nil {
			return ErrCannotReinitialize
		}
	} else if !errors.Is(err, gitinterface.ErrReferenceNotFound) {
		return err
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

// GetRepositoryLocation returns the canonical location of the Git repository
func (r *Repository) GetRepositoryLocation(ctx context.Context) (string, error) {
	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r, policy.PolicyStagingRef, policyopts.BypassRSL())
	if err != nil {
		return "", err
	}

	rootMetadata, err := state.GetRootMetadata(true)
	if err != nil {
		return "", err
	}

	location := rootMetadata.GetRepositoryLocation()

	return location, nil
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

// GetPrimaryRuleFilePrincipals returns the principals trusted for the primary
// rule file.
func (r *Repository) GetPrimaryRuleFilePrincipals(ctx context.Context) ([]tuf.Principal, error) {
	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r, policy.PolicyStagingRef)
	if err != nil {
		return nil, err
	}

	rootMetadata, err := state.GetRootMetadata(true)
	if err != nil {
		return nil, err
	}

	principals, err := rootMetadata.GetPrimaryRuleFilePrincipals()
	if err != nil {
		return nil, err
	}

	return principals, nil
}

// AddGitHubApp is the interface for the user to add the authorized key for the
// trusted GitHub app. This key is used to verify GitHub pull request approval
// attestation signatures recorded by the app.
func (r *Repository) AddGitHubApp(ctx context.Context, signer sslibdsse.SignerVerifier, appName string, appKey tuf.Principal, signCommit bool, opts ...trustpolicyopts.Option) error {
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
	if appName == "" {
		slog.Debug(fmt.Sprintf("Using default app name '%s'...", tuf.GitHubAppRoleName))
		appName = tuf.GitHubAppRoleName
	}
	if err := rootMetadata.AddGitHubAppPrincipal(appName, appKey); err != nil {
		return fmt.Errorf("failed to add GitHub app key: %w", err)
	}

	commitMessage := fmt.Sprintf("Add GitHub app key '%s' to root", appKey.ID())
	return r.updateRootMetadata(ctx, state, signer, rootMetadata, commitMessage, options.CreateRSLEntry, signCommit)
}

// RemoveGitHubApp is the interface for the user to de-authorize the key for the
// special GitHub app role.
func (r *Repository) RemoveGitHubApp(ctx context.Context, signer sslibdsse.SignerVerifier, appName string, signCommit bool, opts ...trustpolicyopts.Option) error {
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
	if appName == "" {
		slog.Debug(fmt.Sprintf("Using default app name '%s'...", tuf.GitHubAppRoleName))
		appName = tuf.GitHubAppRoleName
	}
	rootMetadata.DeleteGitHubAppPrincipal(appName)

	commitMessage := "Remove GitHub app key from root"
	return r.updateRootMetadata(ctx, state, signer, rootMetadata, commitMessage, options.CreateRSLEntry, signCommit)
}

// TrustGitHubApp updates the root metadata to mark GitHub app pull request
// approvals as trusted.
func (r *Repository) TrustGitHubApp(ctx context.Context, signer sslibdsse.SignerVerifier, appName string, signCommit bool, opts ...trustpolicyopts.Option) error {
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

	if appName == "" {
		slog.Debug(fmt.Sprintf("Using default app name '%s'...", tuf.GitHubAppRoleName))
		appName = tuf.GitHubAppRoleName
	}
	if rootMetadata.IsGitHubAppApprovalTrusted(appName) {
		slog.Debug("GitHub app approvals are already trusted, exiting...")
		return nil
	}

	slog.Debug("Marking GitHub app approvals as trusted in root...")
	rootMetadata.EnableGitHubAppApprovals(appName)

	commitMessage := "Mark GitHub app approvals as trusted"
	return r.updateRootMetadata(ctx, state, signer, rootMetadata, commitMessage, options.CreateRSLEntry, signCommit)
}

// UntrustGitHubApp updates the root metadata to mark GitHub app pull request
// approvals as untrusted.
func (r *Repository) UntrustGitHubApp(ctx context.Context, signer sslibdsse.SignerVerifier, appName string, signCommit bool, opts ...trustpolicyopts.Option) error {
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

	if appName == "" {
		slog.Debug(fmt.Sprintf("Using default app name '%s'...", tuf.GitHubAppRoleName))
		appName = tuf.GitHubAppRoleName
	}
	if !rootMetadata.IsGitHubAppApprovalTrusted(appName) {
		slog.Debug("GitHub app approvals are already untrusted, exiting...")
		return nil
	}

	slog.Debug("Marking GitHub app approvals as untrusted in root...")
	rootMetadata.DisableGitHubAppApprovals(appName)

	commitMessage := "Mark GitHub app approvals as untrusted"
	return r.updateRootMetadata(ctx, state, signer, rootMetadata, commitMessage, options.CreateRSLEntry, signCommit)
}

// AreGitHubAppApprovalsTrusted returns which of the GitHub apps defined in the
// metadata are trusted.
func (r *Repository) AreGitHubAppApprovalsTrusted(ctx context.Context, opts ...trustpolicyopts.Option) (map[string]bool, error) {
	options := &trustpolicyopts.Options{}
	for _, fn := range opts {
		fn(options)
	}

	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r, policy.PolicyStagingRef, policyopts.BypassRSL())
	if err != nil {
		return nil, err
	}

	rootMetadata, err := state.GetRootMetadata(true)
	if err != nil {
		return nil, err
	}

	appNames, err := rootMetadata.GetGitHubAppEntries()
	if err != nil {
		return nil, err
	}

	var appsStatus = make(map[string]bool)
	for appName, entry := range appNames {
		appsStatus[appName] = entry.IsTrusted()
	}

	return appsStatus, nil
}

// GetGitHubAppPrincipals returns the principals used for GitHub app
// attestations.
func (r *Repository) GetGitHubAppPrincipals(ctx context.Context) (map[string][]tuf.Principal, error) {
	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r, policy.PolicyStagingRef, policyopts.BypassRSL())
	if err != nil {
		return nil, err
	}

	rootMetadata, err := state.GetRootMetadata(true)
	if err != nil {
		return nil, err
	}

	appNames, err := rootMetadata.GetGitHubAppEntries()
	if err != nil {
		return nil, err
	}

	var appPrincipals = make(map[string][]tuf.Principal)

	for appName := range appNames {
		principals, err := rootMetadata.GetGitHubAppPrincipals(appName)
		if err != nil {
			return nil, err
		}
		appPrincipals[appName] = principals
	}

	return appPrincipals, nil
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

// GetRootThreshold retrieves the threshold of valid signatures required for
// changes to the root of trust
func (r *Repository) GetRootThreshold(ctx context.Context) (int, error) {
	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r, policy.PolicyStagingRef, policyopts.BypassRSL())
	if err != nil {
		return -1, err
	}

	rootMetadata, err := state.GetRootMetadata(true)
	if err != nil {
		return -1, err
	}

	slog.Debug("Getting root threshold...")
	rootThreshold, err := rootMetadata.GetRootThreshold()
	if err != nil {
		return -1, err
	}

	return rootThreshold, nil
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

// GetTopLevelTargetsThreshold returns the threshold of approvals needed to make changes to the primary rule file.
func (r *Repository) GetTopLevelTargetsThreshold(ctx context.Context) (int, error) {
	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r, policy.PolicyStagingRef)
	if err != nil {
		return -1, err
	}

	rootMetadata, err := state.GetRootMetadata(true)
	if err != nil {
		return -1, err
	}

	threshold, err := rootMetadata.GetPrimaryRuleFileThreshold()
	if err != nil {
		return -1, err
	}

	return threshold, nil
}

// AddGlobalRuleThreshold adds a threshold global rule to the root metadata.
func (r *Repository) AddGlobalRuleThreshold(ctx context.Context, signer sslibdsse.SignerVerifier, name string, patterns []string, threshold int, signCommit bool, opts ...trustpolicyopts.Option) error {
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

func (r *Repository) AddPropagationDirective(ctx context.Context, signer sslibdsse.SignerVerifier, directiveName, upstreamRepository, upstreamReference, upstreamPath, downstreamReference, downstreamPath string, signCommit bool, opts ...trustpolicyopts.Option) error {
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
		directive = tufv01.NewPropagationDirective(directiveName, upstreamRepository, upstreamReference, upstreamPath, downstreamReference, downstreamPath)
	case *tufv02.RootMetadata:
		directive = tufv02.NewPropagationDirective(directiveName, upstreamRepository, upstreamReference, upstreamPath, downstreamReference, downstreamPath)
	}

	if err := rootMetadata.AddPropagationDirective(directive); err != nil {
		return err
	}

	commitMessage := fmt.Sprintf("Add propagation directive '%s' to root metadata", directiveName)
	return r.updateRootMetadata(ctx, state, signer, rootMetadata, commitMessage, options.CreateRSLEntry, signCommit)
}

func (r *Repository) UpdatePropagationDirective(ctx context.Context, signer sslibdsse.SignerVerifier, directiveName, upstreamRepository, upstreamReference, upstreamPath, downstreamReference, downstreamPath string, signCommit bool, opts ...trustpolicyopts.Option) error {
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

	slog.Debug("Updating propagation directive...")
	var directive tuf.PropagationDirective
	switch rootMetadata.(type) {
	case *tufv01.RootMetadata:
		directive = tufv01.NewPropagationDirective(directiveName, upstreamRepository, upstreamReference, upstreamPath, downstreamReference, downstreamPath)
	case *tufv02.RootMetadata:
		directive = tufv02.NewPropagationDirective(directiveName, upstreamRepository, upstreamReference, upstreamPath, downstreamReference, downstreamPath)
	}

	if err := rootMetadata.UpdatePropagationDirective(directive); err != nil {
		return err
	}

	commitMessage := fmt.Sprintf("Update propagation directive '%s' in root metadata", directiveName)
	return r.updateRootMetadata(ctx, state, signer, rootMetadata, commitMessage, options.CreateRSLEntry, signCommit)
}

func (r *Repository) RemovePropagationDirective(ctx context.Context, signer sslibdsse.SignerVerifier, name string, signCommit bool, opts ...trustpolicyopts.Option) error {
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
func (r *Repository) AddHook(ctx context.Context, signer sslibdsse.SignerVerifier, stages []tuf.HookStage, hookName string, hookBytes []byte, environment tuf.HookEnvironment, principalIDs []string, timeout int, signCommit bool, opts ...trustpolicyopts.Option) error {
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

	if timeout < 1 {
		return ErrInvalidHookTimeout
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
	hook, err := rootMetadata.AddHook(stages, hookName, principalIDs, hashes, environment, timeout)
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

// UpdateHook updates the hook specified by stage and hookName with the new
// principalIDs, hashes, environment, and timeout.
func (r *Repository) UpdateHook(ctx context.Context, signer sslibdsse.SignerVerifier, stages []tuf.HookStage, hookName string, hookBytes []byte, environment tuf.HookEnvironment, principalIDs []string, timeout int, signCommit bool, opts ...trustpolicyopts.Option) error {
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

	if timeout < 1 {
		return ErrInvalidHookTimeout
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
	hashes[gitinterface.GitBlobHashName] = blobID.String()

	sha256Hash := sha256.New()
	sha256Hash.Write(hookBytes)
	hashes[gitinterface.SHA256HashName] = hex.EncodeToString(sha256Hash.Sum(nil))

	slog.Debug("Updating hook in rule file...")
	err = rootMetadata.UpdateHook(stages, hookName, principalIDs, hashes, environment, timeout)
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

	commitMessage := fmt.Sprintf("Update hook '%s' in root metadata", hookName)
	return r.updateRootMetadata(ctx, state, signer, rootMetadata, commitMessage, options.CreateRSLEntry, signCommit)
}

// EnableController makes the current repository a "controller" repository used
// to specify gittuf policies for other repositories.
func (r *Repository) EnableController(ctx context.Context, signer sslibdsse.SignerVerifier, signCommit bool, opts ...trustpolicyopts.Option) error {
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
	rootMetadata, err := state.GetRootMetadata(true)
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
