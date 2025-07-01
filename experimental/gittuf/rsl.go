// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gittuf

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	rslopts "github.com/gittuf/gittuf/experimental/gittuf/options/rsl"
	"github.com/gittuf/gittuf/internal/common/set"
	"github.com/gittuf/gittuf/internal/dev"
	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/gittuf/gittuf/internal/tuf"
)

const gittufTransportPrefix = "gittuf::"

var (
	ErrCommitNotInRef              = errors.New("specified commit is not in ref")
	ErrPushingRSL                  = errors.New("unable to push RSL")
	ErrPullingRSL                  = errors.New("unable to pull RSL")
	ErrDivergedRefs                = errors.New("references in local repository have diverged from upstream")
	ErrRemoteNotSpecified          = errors.New("remote not specified")
	ErrCannotUseRemoteAndLocalOnly = errors.New("cannot indicate local-only and push to specified remote")
)

// RecordRSLEntryForReference is the interface for the user to add an RSL entry
// for the specified Git reference.
func (r *Repository) RecordRSLEntryForReference(ctx context.Context, refName string, signCommit bool, opts ...rslopts.RecordOption) error {
	if signCommit {
		slog.Debug("Checking if Git signing is configured...")
		err := r.r.CanSign()
		if err != nil {
			return err
		}
	}

	options := &rslopts.RecordOptions{}
	for _, fn := range opts {
		fn(options)
	}

	if options.RemoteName == "" && !options.LocalOnly {
		return ErrRemoteNotSpecified
	} else if options.RemoteName != "" && options.LocalOnly {
		return ErrCannotUseRemoteAndLocalOnly
	}

	if !options.LocalOnly {
		_, err := r.Sync(ctx, options.RemoteName, false, signCommit)
		if err != nil {
			return err
		}
	}

	slog.Debug("Identifying absolute reference path...")
	refName, err := r.r.AbsoluteReference(refName)
	if err != nil {
		return err
	}

	// Track localRefName to check the expected tip as we may override refName
	localRefName := refName

	if options.RefNameOverride != "" {
		// dst differs from src
		// Eg: git push <remote> <src>:<dst>
		slog.Debug("Name of reference overridden to match remote reference name, identifying absolute reference path...")
		refNameOverride, err := r.r.AbsoluteReference(options.RefNameOverride)
		if err != nil {
			return err
		}

		refName = refNameOverride
	}

	// The tip of the ref is always from the localRefName
	slog.Debug(fmt.Sprintf("Loading current state of '%s'...", localRefName))
	refTip, err := r.r.GetReference(localRefName)
	if err != nil {
		return err
	}

	if !options.SkipCheckForDuplicate {
		slog.Debug("Checking if latest entry for reference has same target...")
		isDuplicate, err := r.isDuplicateEntry(refName, refTip)
		if err != nil {
			return err
		}
		if isDuplicate {
			slog.Debug("The latest entry has the same target, skipping creation of new entry...")
			return nil
		}
	} else {
		slog.Debug("Not checking if latest entry for reference has same target")
	}

	// TODO: once policy verification is in place, the signing key used by
	// signCommit must be verified for the refName in the delegation tree.

	slog.Debug("Creating RSL reference entry...")
	if err := rsl.NewReferenceEntry(refName, refTip).Commit(r.r, signCommit); err != nil {
		return err
	}

	if options.LocalOnly {
		return nil
	}

	_, err = r.Sync(ctx, options.RemoteName, false, signCommit)
	return err
}

// RecordRSLEntryForReferenceAtTarget is a special version of
// RecordRSLEntryForReference used for evaluation. It is only invoked when
// gittuf is explicitly set in developer mode.
func (r *Repository) RecordRSLEntryForReferenceAtTarget(refName, targetID string, signingKeyBytes []byte, opts ...rslopts.RecordOption) error {
	// Double check that gittuf is in developer mode
	if !dev.InDevMode() {
		return dev.ErrNotInDevMode
	}

	options := &rslopts.RecordOptions{}
	for _, fn := range opts {
		fn(options)
	}

	slog.Debug("Identifying absolute reference path...")
	refName, err := r.r.AbsoluteReference(refName)
	if err != nil {
		return err
	}

	targetIDHash, err := gitinterface.NewHash(targetID)
	if err != nil {
		return err
	}

	if options.RefNameOverride != "" {
		// dst differs from src
		// Eg: git push <remote> <src>:<dst>
		slog.Debug("Name of reference overridden to match remote reference name, identifying absolute reference path...")
		refName, err = r.r.AbsoluteReference(options.RefNameOverride)
		if err != nil {
			return err
		}
	}

	// TODO: once policy verification is in place, the signing key used by
	// signCommit must be verified for the refName in the delegation tree.

	slog.Debug("Creating RSL reference entry...")
	return rsl.NewReferenceEntry(refName, targetIDHash).CommitUsingSpecificKey(r.r, signingKeyBytes)
}

func (r *Repository) SkipAllInvalidReferenceEntriesForRef(targetRef string, signCommit bool) error {
	return rsl.SkipAllInvalidReferenceEntriesForRef(r.r, targetRef, signCommit)
}

// RecordRSLAnnotation is the interface for the user to add an RSL annotation
// for one or more prior RSL entries.
func (r *Repository) RecordRSLAnnotation(ctx context.Context, rslEntryIDs []string, skip bool, message string, signCommit bool, opts ...rslopts.AnnotateOption) error {
	// TODO: local only?
	if signCommit {
		slog.Debug("Checking if Git signing is configured...")
		err := r.r.CanSign()
		if err != nil {
			return err
		}
	}

	options := &rslopts.AnnotateOptions{}
	for _, fn := range opts {
		fn(options)
	}

	if options.RemoteName == "" && !options.LocalOnly {
		return ErrRemoteNotSpecified
	} else if options.RemoteName != "" && options.LocalOnly {
		return ErrCannotUseRemoteAndLocalOnly
	}

	if !options.LocalOnly {
		_, err := r.Sync(ctx, options.RemoteName, false, signCommit)
		if err != nil {
			return err
		}
	}

	rslEntryHashes := []gitinterface.Hash{}
	for _, id := range rslEntryIDs {
		hash, err := gitinterface.NewHash(id)
		if err != nil {
			return err
		}
		rslEntryHashes = append(rslEntryHashes, hash)
	}

	// TODO: once policy verification is in place, the signing key used by
	// signCommit must be verified for the refNames of the rslEntryIDs.

	slog.Debug("Creating RSL annotation entry...")
	if err := rsl.NewAnnotationEntry(rslEntryHashes, skip, message).Commit(r.r, signCommit); err != nil {
		return err
	}

	if options.LocalOnly {
		return nil
	}

	_, err := r.Sync(ctx, options.RemoteName, false, signCommit)
	return err
}

// ReconcileLocalRSLWithRemote checks the local RSL against the specified remote
// and reconciles the local RSL if needed. If the local RSL doesn't exist or is
// strictly behind the remote RSL, then the local RSL is updated to match the
// remote RSL. If the local RSL is ahead of the remote RSL, nothing is updated.
// Finally, if the local and remote RSLs have diverged, then the local only RSL
// entries are reapplied over the latest entries in the remote if the local only
// RSL entries and remote only entries are for different Git references.
func (r *Repository) ReconcileLocalRSLWithRemote(ctx context.Context, remoteName string, sign bool) error {
	if sign {
		slog.Debug("Checking if Git signing is configured...")
		err := r.r.CanSign()
		if err != nil {
			return err
		}
	}

	remoteURL, err := r.r.GetRemoteURL(remoteName)
	if err != nil {
		return err
	}
	if strings.HasPrefix(remoteURL, gittufTransportPrefix) {
		slog.Debug("Creating new remote to avoid using gittuf transport...")
		remoteName = fmt.Sprintf("check-remote-%s", remoteName)
		if err := r.r.AddRemote(remoteName, strings.TrimPrefix(remoteURL, gittufTransportPrefix)); err != nil {
			return err
		}
		defer r.r.RemoveRemote(remoteName) //nolint:errcheck
	}

	// Fetch status of RSL on the remote
	trackerRef := rsl.RemoteTrackerRef(remoteName)
	rslRemoteRefSpec := []string{fmt.Sprintf("%s:%s", rsl.Ref, trackerRef)}

	slog.Debug(fmt.Sprintf("Updating remote RSL tracker (%s)...", rslRemoteRefSpec))
	if err := r.r.FetchRefSpec(remoteName, rslRemoteRefSpec); err != nil {
		return err
	}

	remoteRefState, err := r.r.GetReference(trackerRef)
	if err != nil {
		return err
	}
	slog.Debug(fmt.Sprintf("Remote RSL is at '%s'", remoteRefState.String()))

	// Load status of the local RSL for comparison
	localRefState, err := r.r.GetReference(rsl.Ref)
	if err != nil {
		return err
	}
	slog.Debug(fmt.Sprintf("Local RSL is at '%s'", localRefState.String()))

	// Check if local is nil and exit appropriately
	if localRefState.IsZero() {
		// Local RSL has not been populated but remote is not zero
		// Fetch updates to the local RSL
		slog.Debug("Local RSL has not been initialized but remote RSL exists, fetching remote RSL...")
		if err := r.r.Fetch(remoteName, []string{rsl.Ref}, true); err != nil {
			return err
		}

		slog.Debug("Updated local RSL!")
		return nil
	}

	// Check if equal and exit early if true
	if remoteRefState.Equal(localRefState) {
		slog.Debug("Local and remote RSLs have same state, nothing to do")
		return nil
	}

	// Next, check if remote is ahead of local
	knows, err := r.r.KnowsCommit(remoteRefState, localRefState)
	if err != nil {
		return err
	}
	if knows {
		slog.Debug("Remote RSL is ahead of local RSL, fetching remote RSL...")
		if err := r.r.Fetch(remoteName, []string{rsl.Ref}, true); err != nil {
			return err
		}

		slog.Debug("Updated local RSL!")
		return nil
	}

	// If not ancestor, local may be ahead or they may have diverged
	// If remote is ancestor, only local is ahead, no updates
	// If remote is not ancestor, the two have diverged, local needs to pull updates
	knows, err = r.r.KnowsCommit(localRefState, remoteRefState)
	if err != nil {
		return err
	}
	if knows {
		// We don't push to the remote RSL, that's handled alongside
		// other pushes (eg. via the transport) or explicitly
		slog.Debug("Local RSL is ahead of remote RSL, nothing to do")
		return nil
	}

	// This is the tricky one
	// First, we find a common ancestor for the two
	// Second, we identify all the entries in the local that is not in the
	// remote
	// Third, we set local to the remote's tip
	// Fourth, we apply all the entries that we identified over the new tip
	slog.Debug("Local and remote RSLs have diverged, identifying common ancestor to reconcile local RSL...")
	commonAncestor, err := r.r.GetCommonAncestor(localRefState, remoteRefState)
	if err != nil {
		return err
	}
	slog.Debug(fmt.Sprintf("Found common ancestor entry '%s'", commonAncestor.String()))

	localOnlyEntries, err := getRSLEntriesUntil(r.r, localRefState, commonAncestor)
	if err != nil {
		return err
	}
	remoteOnlyEntries, err := getRSLEntriesUntil(r.r, remoteRefState, commonAncestor)
	if err != nil {
		return err
	}

	localUpdatedRefs := set.NewSet[string]()
	for _, entry := range localOnlyEntries {
		slog.Debug(fmt.Sprintf("Identified local only entry that must be reapplied '%s'", entry.GetID().String()))
		if entry, isRefEntry := entry.(*rsl.ReferenceEntry); isRefEntry {
			localUpdatedRefs.Add(entry.RefName)
		}
	}

	remoteUpdatedRefs := set.NewSet[string]()
	for _, entry := range remoteOnlyEntries {
		slog.Debug(fmt.Sprintf("Identified remote only entry '%s'", entry.GetID().String()))
		if entry, isRefEntry := entry.(*rsl.ReferenceEntry); isRefEntry {
			remoteUpdatedRefs.Add(entry.RefName)
		}
	}

	// Check if remote has entries for refs that are also updated locally
	// We don't want to do conflict resolution right now
	intersection := localUpdatedRefs.Intersection(remoteUpdatedRefs)
	if intersection.Len() != 0 {
		return fmt.Errorf("unable to reconcile local RSL with remote; both RSLs contain changes to the same refs [%s]", strings.Join(intersection.Contents(), ", "))
	}

	// Set local RSL to match the remote state
	if err := r.r.SetReference(rsl.Ref, remoteRefState); err != nil {
		return fmt.Errorf("unable to update local RSL: %w", err)
	}

	// Apply local only entries on top of the new local RSL
	// localOnlyEntries is in reverse order
	for i := len(localOnlyEntries) - 1; i >= 0; i-- {
		slog.Debug(fmt.Sprintf("Reapplying entry '%s'...", localOnlyEntries[i].GetID().String()))

		// We create a new object so as to apply anything the
		// entry may contain that is inferred at commit time
		// For example, an incrementing number inferred from the
		// parent entry
		switch entry := localOnlyEntries[i].(type) {
		case *rsl.ReferenceEntry:
			if err := rsl.NewReferenceEntry(entry.RefName, entry.TargetID).Commit(r.r, sign); err != nil {
				return fmt.Errorf("unable to reapply reference entry '%s': %w", entry.ID.String(), err)
			}
		case *rsl.AnnotationEntry:
			if err := rsl.NewAnnotationEntry(entry.RSLEntryIDs, entry.Skip, entry.Message).Commit(r.r, sign); err != nil {
				return fmt.Errorf("unable to reapply annotation entry '%s': %w", entry.ID.String(), err)
			}
		}

		if slog.Default().Enabled(ctx, slog.LevelDebug) {
			currentTip, err := r.r.GetReference(rsl.Ref)
			if err != nil {
				return fmt.Errorf("unable to get current tip of the RSL: %w", err)
			}
			slog.Debug("New entry ID for '%s' is '%s'", localOnlyEntries[i].GetID().String(), currentTip.String())
		}
	}

	slog.Debug("Updated local RSL!")
	return nil
}

// Sync is responsible for synchronizing references between the local copy of
// the repository and the specified remote.
func (r *Repository) Sync(ctx context.Context, remoteName string, overwriteLocalRefs, signCommit bool) ([]string, error) {
	if divergedRefs, err := r.sync(remoteName, overwriteLocalRefs); err != nil {
		return divergedRefs, err
	}

	if err := r.PropagateChangesFromUpstreamRepositories(ctx, signCommit); err != nil {
		return nil, err
	}

	return r.sync(remoteName, overwriteLocalRefs)
}

func (r *Repository) sync(remoteName string, overwriteLocalRefs bool) ([]string, error) {
	remoteURL, err := r.r.GetRemoteURL(remoteName)
	if err != nil {
		return nil, err
	}
	if strings.HasPrefix(remoteURL, gittufTransportPrefix) {
		slog.Debug("Creating new remote to avoid using gittuf transport...")
		remoteName = fmt.Sprintf("check-remote-%s", remoteName)
		if err := r.r.AddRemote(remoteName, strings.TrimPrefix(remoteURL, gittufTransportPrefix)); err != nil {
			return nil, err
		}
		defer r.r.RemoveRemote(remoteName) //nolint:errcheck
	}

	// Fetch status of RSL on the remote
	trackerRef := rsl.RemoteTrackerRef(remoteName)
	rslRemoteRefSpec := []string{fmt.Sprintf("%s:%s", rsl.Ref, trackerRef)}

	slog.Debug(fmt.Sprintf("Updating remote RSL tracker (%s)...", rslRemoteRefSpec))
	if err := r.r.FetchRefSpec(remoteName, rslRemoteRefSpec); err != nil {
		return nil, err
	}
	defer r.r.DeleteReference(trackerRef) //nolint:errcheck

	remoteRefState, err := r.r.GetReference(trackerRef)
	if err != nil {
		return nil, err
	}
	slog.Debug(fmt.Sprintf("Remote RSL is at '%s'", remoteRefState.String()))

	// Load status of the local RSL for comparison
	localRefState, err := r.r.GetReference(rsl.Ref)
	if err != nil {
		return nil, err
	}
	slog.Debug(fmt.Sprintf("Local RSL is at '%s'", localRefState.String()))

	// Check if equal and exit early if true
	if remoteRefState.Equal(localRefState) {
		slog.Debug("Local and remote RSLs have same state, nothing to do")
		// non error exit
		return nil, nil
	}

	// local RSL is ahead of remote RSL
	slog.Debug("Checking if local RSL is ahead of remote RSL...")
	localAheadOfRemote, err := r.r.KnowsCommit(localRefState, remoteRefState)
	if err != nil {
		return nil, err
	}
	if localAheadOfRemote {
		slog.Debug("Local RSL is ahead of remote RSL, pushing all locally modified references...")
		localOnlyEntries, err := getRSLEntriesUntil(r.r, localRefState, remoteRefState)
		if err != nil {
			slog.Debug("Unable to identify new entries in local RSL, aborting...")
			return nil, err
		}

		localUpdatedRefTips := getLatestRefTipsFromRSLEntries(localOnlyEntries)
		pushRefs := []string{rsl.Ref}
		for refName := range localUpdatedRefTips {
			pushRefs = append(pushRefs, refName)
		}

		if err := r.r.Push(remoteName, pushRefs); err != nil {
			return nil, err
		}

		slog.Debug("Pushed local changes to remote successfully!")
		return nil, nil
	}

	// remote RSL is ahead of local RSL -> check if any local ref changes
	// conflict and display message to user
	slog.Debug("Checking if remote RSL is ahead of local RSL...")
	remoteAheadOfLocal, err := r.r.KnowsCommit(remoteRefState, localRefState)
	if err != nil {
		return nil, err
	}
	if remoteAheadOfLocal {
		slog.Debug("Remote RSL is ahead of local RSL")
		// Track the latest tips in the remote RSL using the entries that are new
		// compared to the local RSL
		remoteOnlyEntries, err := getRSLEntriesUntil(r.r, remoteRefState, localRefState)
		if err != nil {
			slog.Debug("Unable to identify new entries in remote RSL, aborting...")
			return nil, err
		}

		remoteUpdatedRefTips := getLatestRefTipsFromRSLEntries(remoteOnlyEntries)

		referenceUpdateDirectives := map[string]gitinterface.Hash{
			rsl.Ref: remoteRefState,
		}
		divergedRefs := []string{}
		for refName, remoteTip := range remoteUpdatedRefTips {
			// Find local tip for same ref
			slog.Debug(fmt.Sprintf("Inspecting state of '%s' locally...", refName))
			localTip, err := r.r.GetReference(refName)
			if err != nil {
				if !errors.Is(err, gitinterface.ErrReferenceNotFound) {
					return nil, err
				}

				slog.Debug(fmt.Sprintf("Reference '%s' does not exist locally", refName))
				continue
			}

			// Fetch remote objects for each ref
			if !r.r.HasObject(remoteTip) {
				if err := r.r.FetchObject(remoteName, remoteTip); err != nil {
					slog.Debug(fmt.Sprintf("Unable to fetch object '%s', aborting...", remoteTip.String()))
					return nil, err
				}
			}

			// Now we actually have remoteTip in the object store
			objType, err := r.r.GetObjectType(remoteTip)
			if err != nil {
				return nil, fmt.Errorf("unable to inspect object '%s': %w", remoteTip.String(), err)
			}
			switch objType {
			case gitinterface.CommitObjectType:
				// if remoteTip is ahead of localTip, we're good
				// otherwise, mark that ref as candidate for overwriting locally
				remoteAheadOfLocal, err := r.r.KnowsCommit(remoteTip, localTip)
				if err != nil {
					return nil, err
				}
				if remoteAheadOfLocal {
					referenceUpdateDirectives[refName] = remoteTip
				} else {
					divergedRefs = append(divergedRefs, refName)
				}
			default:
				// If tags (or other ref->obj mappings) are not equal, mark that
				// ref as candidate for overwriting locally
				if !remoteTip.Equal(localTip) {
					divergedRefs = append(divergedRefs, refName)
				}
			}
		}

		if len(divergedRefs) != 0 {
			if !overwriteLocalRefs {
				slog.Debug(fmt.Sprintf("Local references have diverged from upstream repository: [%s]", strings.Join(divergedRefs, ", ")))
				return divergedRefs, ErrDivergedRefs
			}

			for _, refName := range divergedRefs {
				remoteTip := remoteUpdatedRefTips[refName]
				referenceUpdateDirectives[refName] = remoteTip
			}
		}

		for refName, tip := range referenceUpdateDirectives {
			if err := r.r.SetReference(refName, tip); err != nil {
				return nil, err
			}
		}

		// non error exit
		// TODO: restore worktree if checked out HEAD is in divergedRefs
		return nil, nil
	}

	// The RSL itself has diverged
	// We can't fix this if overwriteLocalRefs is not true
	slog.Debug("Local and remote RSLs have diverged...")
	if !overwriteLocalRefs {
		slog.Debug("Cannot reconcile local and remote RSLs as overwriting local changes is disallowed, aborting...")
		return []string{rsl.Ref}, ErrDivergedRefs
	}

	commonAncestor, err := r.r.GetCommonAncestor(localRefState, remoteRefState)
	if err != nil {
		return nil, err
	}
	slog.Debug(fmt.Sprintf("Found common ancestor entry '%s'", commonAncestor.String()))

	remoteOnlyEntries, err := getRSLEntriesUntil(r.r, remoteRefState, commonAncestor)
	if err != nil {
		return nil, err
	}

	remoteUpdatedRefTips := getLatestRefTipsFromRSLEntries(remoteOnlyEntries)

	referenceUpdateDirectives := map[string]gitinterface.Hash{
		rsl.Ref: remoteRefState,
	}
	divergedRefs := []string{}
	for refName, remoteTip := range remoteUpdatedRefTips {
		// Find local tip for same ref
		slog.Debug(fmt.Sprintf("Inspecting state of '%s' locally...", refName))
		localTip, err := r.r.GetReference(refName)
		if err != nil {
			if !errors.Is(err, gitinterface.ErrReferenceNotFound) {
				return nil, err
			}

			slog.Debug(fmt.Sprintf("Reference '%s' does not exist locally", refName))
			continue
		}

		// Fetch remote objects for each ref
		if !r.r.HasObject(remoteTip) {
			if err := r.r.FetchObject(remoteName, remoteTip); err != nil {
				slog.Debug(fmt.Sprintf("Unable to fetch object '%s', aborting...", remoteTip.String()))
				return nil, err
			}
		}

		// Now we actually have remoteTip in the object store
		objType, err := r.r.GetObjectType(remoteTip)
		if err != nil {
			return nil, fmt.Errorf("unable to inspect object '%s': %w", remoteTip.String(), err)
		}
		switch objType {
		case gitinterface.CommitObjectType:
			// if remoteTip is ahead of localTip, we're good
			// otherwise, mark that ref as candidate for overwriting locally
			remoteAheadOfLocal, err := r.r.KnowsCommit(remoteTip, localTip)
			if err != nil {
				return nil, err
			}
			if remoteAheadOfLocal {
				referenceUpdateDirectives[refName] = remoteTip
			} else {
				divergedRefs = append(divergedRefs, refName)
			}
		default:
			// If tags (or other ref->obj mappings) are not equal, mark that
			// ref as candidate for overwriting locally
			if !remoteTip.Equal(localTip) {
				divergedRefs = append(divergedRefs, refName)
			}
		}
	}

	for _, refName := range divergedRefs {
		remoteTip := remoteUpdatedRefTips[refName]
		referenceUpdateDirectives[refName] = remoteTip
	}

	for refName, expectedTip := range referenceUpdateDirectives {
		if err := r.r.SetReference(refName, expectedTip); err != nil {
			return nil, fmt.Errorf("unable to update local reference '%s'", refName)
		}
	}

	// non error exit
	slog.Debug("Updated local RSL!")
	return nil, nil
}

func getRSLEntriesUntil(repo *gitinterface.Repository, start, until gitinterface.Hash) ([]rsl.Entry, error) {
	entries := []rsl.Entry{}

	iterator, err := rsl.GetEntry(repo, start)
	if err != nil {
		return nil, fmt.Errorf("unable to load entry '%s': %w", start.String(), err)
	}

	for {
		entries = append(entries, iterator)

		parent, err := rsl.GetParentForEntry(repo, iterator)
		if err != nil {
			return nil, fmt.Errorf("unable to load parent of entry '%s': %w", iterator.GetID().String(), err)
		}

		if parent.GetID().Equal(until) {
			break
		}

		iterator = parent
	}

	return entries, nil
}

func getLatestRefTipsFromRSLEntries(entries []rsl.Entry) map[string]gitinterface.Hash {
	refTips := map[string]gitinterface.Hash{}
	annotationsMap := map[string][]*rsl.AnnotationEntry{}
	for _, entry := range entries {
		switch entry := entry.(type) {
		case *rsl.ReferenceEntry:
			if _, has := refTips[entry.GetRefName()]; has {
				continue
			}

			annotations, has := annotationsMap[entry.GetID().String()]
			if has && entry.SkippedBy(annotations) {
				continue
			}

			refTips[entry.GetRefName()] = entry.GetTargetID()
		case *rsl.PropagationEntry:
			if _, has := refTips[entry.GetRefName()]; has {
				continue
			}
		case *rsl.AnnotationEntry:
			for _, referencedEntryID := range entry.RSLEntryIDs {
				if _, has := annotationsMap[referencedEntryID.String()]; !has {
					annotationsMap[referencedEntryID.String()] = []*rsl.AnnotationEntry{}
				}
				annotationsMap[referencedEntryID.String()] = append(annotationsMap[referencedEntryID.String()], entry)
			}
		}
	}

	return refTips
}

// PushRSL pushes the local RSL to the specified remote. As this push defaults
// to fast-forward only, divergent RSL states are detected.
func (r *Repository) PushRSL(remoteName string) error {
	slog.Debug(fmt.Sprintf("Pushing RSL reference to '%s'...", remoteName))
	if err := r.r.Push(remoteName, []string{rsl.Ref}); err != nil {
		return errors.Join(ErrPushingRSL, err)
	}

	return nil
}

// PullRSL pulls RSL contents from the specified remote to the local RSL. The
// fetch is marked as fast forward only to detect RSL divergence.
func (r *Repository) PullRSL(remoteName string) error {
	slog.Debug(fmt.Sprintf("Pulling RSL reference from '%s'...", remoteName))
	if err := r.r.Fetch(remoteName, []string{rsl.Ref}, true); err != nil {
		return errors.Join(ErrPullingRSL, err)
	}

	return nil
}

// isDuplicateEntry checks if the latest unskipped entry for the ref has the
// same target ID. Note that it's legal for the RSL to have target A, then B,
// then A again, this is not considered a duplicate entry
func (r *Repository) isDuplicateEntry(refName string, targetID gitinterface.Hash) (bool, error) {
	latestUnskippedEntry, _, err := rsl.GetLatestReferenceUpdaterEntry(r.r, rsl.ForReference(refName), rsl.IsUnskipped())
	if err != nil {
		if errors.Is(err, rsl.ErrRSLEntryNotFound) {
			return false, nil
		}
		return false, err
	}

	return latestUnskippedEntry.GetTargetID().Equal(targetID), nil
}

// PropagateChangesFromUpstreamRepositories invokes gittuf's propagation
// workflow. It inspects the latest policy metadata to find the applicable
// propagation directives, and executes the workflow on each one.
func (r *Repository) PropagateChangesFromUpstreamRepositories(ctx context.Context, sign bool) error {
	slog.Debug("Checking if upstream changes must be propagated...")
	state, err := policy.LoadCurrentState(ctx, r.r, policy.PolicyRef)
	if err != nil {
		if errors.Is(err, rsl.ErrRSLEntryNotFound) {
			return nil
		}

		return err
	}

	rootMetadata, err := state.GetRootMetadata(false)
	if err != nil {
		return err
	}
	directives := rootMetadata.GetPropagationDirectives()

	upstreamRepositoryDirectivesMapping := map[string][]tuf.PropagationDirective{}
	for _, directive := range directives {
		// Group directives for the same repository together
		if _, has := upstreamRepositoryDirectivesMapping[directive.GetUpstreamRepository()]; !has {
			upstreamRepositoryDirectivesMapping[directive.GetUpstreamRepository()] = []tuf.PropagationDirective{}
		}

		upstreamRepositoryDirectivesMapping[directive.GetUpstreamRepository()] = append(upstreamRepositoryDirectivesMapping[directive.GetUpstreamRepository()], directive)
	}

	for upstreamRepositoryURL, directives := range upstreamRepositoryDirectivesMapping {
		slog.Debug(fmt.Sprintf("Propagating changes from repository '%s'...", upstreamRepositoryURL))
		upstreamRepositoryLocation, err := os.MkdirTemp("", "gittuf-propagate-upstream")
		if err != nil {
			return err
		}
		defer os.RemoveAll(upstreamRepositoryLocation) //nolint:errcheck

		fetchReferences := set.NewSetFromItems(rsl.Ref)
		for _, directive := range directives {
			fetchReferences.Add(directive.GetUpstreamReference())
		}

		upstreamRepository, err := gitinterface.CloneAndFetchRepository(upstreamRepositoryURL, upstreamRepositoryLocation, "", fetchReferences.Contents(), true)
		if err != nil {
			// TODO: we see this error when required upstream ref isn't found, handle gracefully?
			return fmt.Errorf("unable to fetch upstream repository '%s': %w", upstreamRepositoryURL, err)
		}

		if err := rsl.PropagateChangesFromUpstreamRepository(r.r, upstreamRepository, directives, sign); err != nil {
			// TODO: atomic? abort?
			return err
		}
	}

	return nil
}
