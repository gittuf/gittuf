// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gittuf

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"

	rslopts "github.com/gittuf/gittuf/experimental/gittuf/options/rsl"
	"github.com/gittuf/gittuf/internal/common/set"
	"github.com/gittuf/gittuf/internal/dev"
	"github.com/gittuf/gittuf/internal/policy"
	policyopts "github.com/gittuf/gittuf/internal/policy/options/policy"
	"github.com/gittuf/gittuf/internal/propagation"
	"github.com/gittuf/gittuf/internal/tuf"
	tufv01 "github.com/gittuf/gittuf/internal/tuf/v01"
	"github.com/gittuf/gittuf/pkg/githash"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/gittuf/gittuf/pkg/gitstore"
	"github.com/gittuf/gittuf/pkg/rsl"
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
	options := &rslopts.RecordOptions{}
	for _, fn := range opts {
		fn(options)
	}

	if signCommit && options.SigningKeyBytes == nil {
		slog.Debug("Checking if Git signing is configured...")
		err := r.r.CanSign()
		if err != nil {
			return err
		}
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

	refName, refTip, err := r.resolveReferenceUpdate(refName, options.RefNameOverride)
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
	entry := rsl.NewReferenceEntry(refName, refTip, rsl.WithCustomFields(rsl.CustomFields(options.CustomFields)))
	if signCommit && options.SigningKeyBytes != nil {
		if err := entry.CommitUsingSpecificKey(r.r, options.SigningKeyBytes); err != nil {
			return err
		}
	} else if err := entry.Commit(r.r, signCommit); err != nil {
		return err
	}

	if options.LocalOnly {
		return nil
	}

	_, err = r.Sync(ctx, options.RemoteName, false, signCommit)
	return err
}

// ReferenceUpdateRequest names one local reference to record, optionally
// under a different name, as during git push <src>:<dst>.
type ReferenceUpdateRequest struct {
	RefName         string
	RefNameOverride string
}

// resolvedReferenceUpdate is a request after ref resolution and duplicate
// filtering.
type resolvedReferenceUpdate struct {
	refName string
	tip     githash.Hash
}

// RecordRSLEntryForReferences records the current state of several
// references. When more than one update remains after duplicate filtering,
// one BulkReferenceEntry is recorded, so either every update is recorded by
// that single entry or none of them is. A single remaining update is
// recorded as one ReferenceEntry, exactly as RecordRSLEntryForReference
// does.
func (r *Repository) RecordRSLEntryForReferences(ctx context.Context, updates []ReferenceUpdateRequest, signCommit bool, opts ...rslopts.RecordOption) error {
	options := &rslopts.RecordOptions{}
	for _, fn := range opts {
		fn(options)
	}

	if signCommit && options.SigningKeyBytes == nil {
		slog.Debug("Checking if Git signing is configured...")
		if err := r.r.CanSign(); err != nil {
			return err
		}
	}

	if options.RemoteName == "" && !options.LocalOnly {
		return ErrRemoteNotSpecified
	} else if options.RemoteName != "" && options.LocalOnly {
		return ErrCannotUseRemoteAndLocalOnly
	}

	if !options.LocalOnly {
		if _, err := r.Sync(ctx, options.RemoteName, false, signCommit); err != nil {
			return err
		}
	}

	// A bulk entry must not list the same reference twice, so requests that
	// resolve to the same recorded reference are collapsed into the first
	// position with the last tip given.
	seen := make(map[string]int, len(updates))
	resolved := make([]resolvedReferenceUpdate, 0, len(updates))
	for _, update := range updates {
		refName, tip, err := r.resolveReferenceUpdate(update.RefName, update.RefNameOverride)
		if err != nil {
			return err
		}

		if i, isSeen := seen[refName]; isSeen {
			slog.Debug(fmt.Sprintf("Reference '%s' is named more than once, using the last target...", refName))
			resolved[i].tip = tip
			continue
		}

		seen[refName] = len(resolved)
		resolved = append(resolved, resolvedReferenceUpdate{refName: refName, tip: tip})
	}

	if !options.SkipCheckForDuplicate {
		slog.Debug("Checking if latest entry for each reference has same target...")
		remaining := make([]resolvedReferenceUpdate, 0, len(resolved))
		for _, update := range resolved {
			isDuplicate, err := r.isDuplicateEntry(update.refName, update.tip)
			if err != nil {
				return err
			}
			if isDuplicate {
				slog.Debug(fmt.Sprintf("The latest entry for '%s' has the same target, skipping...", update.refName))
				continue
			}

			remaining = append(remaining, update)
		}
		resolved = remaining
	} else {
		slog.Debug("Not checking if latest entry for each reference has same target")
	}

	if len(resolved) == 0 {
		return nil
	}

	// TODO: once policy verification is in place, the signing key used by
	// signCommit must be verified for each refName in the delegation tree.

	if len(resolved) > 1 {
		slog.Debug("Creating RSL bulk reference entry...")
		rslUpdates := make([]rsl.ReferenceUpdate, 0, len(resolved))
		for _, update := range resolved {
			rslUpdates = append(rslUpdates, rsl.ReferenceUpdate{RefName: update.refName, TargetID: update.tip})
		}
		entry := rsl.NewBulkReferenceEntry(rslUpdates)
		if signCommit && options.SigningKeyBytes != nil {
			if err := entry.CommitUsingSpecificKey(r.r, options.SigningKeyBytes); err != nil {
				return err
			}
		} else if err := entry.Commit(r.r, signCommit); err != nil {
			return err
		}
	} else {
		for _, update := range resolved {
			slog.Debug(fmt.Sprintf("Creating RSL reference entry for '%s'...", update.refName))
			entry := rsl.NewReferenceEntry(update.refName, update.tip)
			if signCommit && options.SigningKeyBytes != nil {
				if err := entry.CommitUsingSpecificKey(r.r, options.SigningKeyBytes); err != nil {
					return err
				}
			} else if err := entry.Commit(r.r, signCommit); err != nil {
				return err
			}
		}
	}

	if options.LocalOnly {
		return nil
	}

	_, err := r.Sync(ctx, options.RemoteName, false, signCommit)
	return err
}

// resolveReferenceUpdate resolves the local ref to record and its tip. The
// tip is always read from the local ref. The recorded name is the override
// when one is given.
func (r *Repository) resolveReferenceUpdate(refName, refNameOverride string) (string, githash.Hash, error) {
	slog.Debug("Identifying absolute reference path...")
	localRefName, err := r.r.AbsoluteReference(refName)
	if err != nil {
		return "", nil, err
	}

	recordedRefName := localRefName
	if refNameOverride != "" {
		// dst differs from src
		// Eg: git push <remote> <src>:<dst>
		slog.Debug("Name of reference overridden to match remote reference name, identifying absolute reference path...")
		recordedRefName, err = r.r.AbsoluteReference(refNameOverride)
		if err != nil {
			return "", nil, err
		}
	}

	// The tip of the ref is always from the localRefName
	slog.Debug(fmt.Sprintf("Loading current state of '%s'...", localRefName))
	tip, err := r.r.GetReference(localRefName)
	if err != nil {
		return "", nil, err
	}

	return recordedRefName, tip, nil
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
	return rsl.NewReferenceEntry(refName, targetIDHash, rsl.WithCustomFields(rsl.CustomFields(options.CustomFields))).CommitUsingSpecificKey(r.r, signingKeyBytes)
}

func (r *Repository) SkipAllInvalidReferenceEntriesForRef(targetRef string, signCommit bool) error {
	if signCommit {
		if err := r.r.CanSign(); err != nil {
			return err
		}
	}

	return rsl.SkipAllInvalidReferenceEntriesForRef(r.r, targetRef, signCommit)
}

// RecordRSLAnnotation is the interface for the user to add an RSL annotation
// for one or more prior RSL entries.
func (r *Repository) RecordRSLAnnotation(ctx context.Context, rslEntryIDs []string, skip bool, message string, signCommit bool, opts ...rslopts.AnnotateOption) error {
	options := &rslopts.AnnotateOptions{}
	for _, fn := range opts {
		fn(options)
	}

	if signCommit && options.SigningKeyBytes == nil {
		slog.Debug("Checking if Git signing is configured...")
		err := r.r.CanSign()
		if err != nil {
			return err
		}
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

	rslEntryHashes := []githash.Hash{}
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
	annotation := rsl.NewAnnotationEntry(rslEntryHashes, skip, message, rsl.WithCustomFields(rsl.CustomFields(options.CustomFields)))
	if signCommit && options.SigningKeyBytes != nil {
		if err := annotation.CommitUsingSpecificKey(r.r, options.SigningKeyBytes); err != nil {
			return err
		}
	} else if err := annotation.Commit(r.r, signCommit); err != nil {
		return err
	}

	if options.LocalOnly {
		return nil
	}

	_, err := r.Sync(ctx, options.RemoteName, false, signCommit)
	return err
}

// rewriteAnnotationTargets maps an annotation's targets and qualifiers
// through the entries a replay wrote. A target that was not replayed keeps
// its ID and its qualifiers. A replayed target is mapped to the ID of the
// entry the replay wrote, with its qualifiers rekeyed to that ID.
func rewriteAnnotationTargets(entry *rsl.AnnotationEntry, replayedIDs map[string]githash.Hash) ([]githash.Hash, map[string][]string) {
	newIDs := make([]githash.Hash, 0, len(entry.RSLEntryIDs))
	newRefs := map[string][]string{}

	for _, id := range entry.RSLEntryIDs {
		refs, qualified := entry.Refs[id.String()]

		newID, wasReplayed := replayedIDs[id.String()]
		if !wasReplayed {
			newID = id
		}

		newIDs = append(newIDs, newID)
		if qualified {
			newRefs[newID.String()] = refs
		}
	}

	return newIDs, newRefs
}

// ReconcileLocalRSLWithRemote checks the local RSL against the specified remote
// and reconciles the local RSL if needed. If the local RSL doesn't exist or is
// strictly behind the remote RSL, then the local RSL is updated to match the
// remote RSL. If the local RSL is ahead of the remote RSL, nothing is updated.
// Finally, if the local and remote RSLs have diverged, then the local only RSL
// entries are reapplied over the latest entries in the remote if the local only
// RSL entries and remote only entries are for different Git references. A
// local only bulk reference entry is replayed as a bulk reference entry.
// Replayed entries receive new IDs, so an annotation that referred to a
// replayed entry is rewritten to the new ID.
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
		if err := addUpdatedRefs(localUpdatedRefs, entry); err != nil {
			return fmt.Errorf("unable to inspect local only entry '%s': %w", entry.GetID().String(), err)
		}
	}

	remoteUpdatedRefs := set.NewSet[string]()
	for _, entry := range remoteOnlyEntries {
		slog.Debug(fmt.Sprintf("Identified remote only entry '%s'", entry.GetID().String()))
		if err := addUpdatedRefs(remoteUpdatedRefs, entry); err != nil {
			return fmt.Errorf("unable to inspect remote only entry '%s': %w", entry.GetID().String(), err)
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

	// Apply local only entries on top of the new local RSL. localOnlyEntries
	// is in reverse order. Replayed entries receive new IDs, so annotations
	// that referred to replayed entries are rewritten to the new IDs.
	replayedIDs := map[string]githash.Hash{}
	for i := len(localOnlyEntries) - 1; i >= 0; i-- {
		original := localOnlyEntries[i]
		slog.DebugContext(ctx, fmt.Sprintf("Reapplying entry '%s'...", original.GetID().String()))

		// We create a new object so as to apply anything the entry may
		// contain that is inferred at commit time, such as the number.
		// WithCustomFields copies the fields into the new entry, so the
		// original entry's map can be passed directly without cloning it
		// first.
		var err error
		switch entry := original.(type) {
		case *rsl.ReferenceEntry:
			err = rsl.NewReferenceEntry(entry.RefName, entry.TargetID, rsl.WithCustomFields(entry.CustomFields)).Commit(r.r, sign)
		case *rsl.BulkReferenceEntry:
			err = rsl.NewBulkReferenceEntry(slices.Clone(entry.Updates)).Commit(r.r, sign)
		case *rsl.PropagationEntry:
			err = rsl.NewPropagationEntry(entry.RefName, entry.TargetID, entry.UpstreamRepository, entry.UpstreamEntryID, rsl.WithCustomFields(entry.CustomFields)).Commit(r.r, sign)
		case *rsl.AnnotationEntry:
			newIDs, newRefs := rewriteAnnotationTargets(entry, replayedIDs)
			err = rsl.NewAnnotationEntryWithQualifiers(newIDs, newRefs, entry.Skip, entry.Message, rsl.WithCustomFields(entry.CustomFields)).Commit(r.r, sign)
		default:
			err = fmt.Errorf("%w: cannot reapply entry type %T", rsl.ErrUnknownRSLEntryType, original)
		}
		if err != nil {
			return fmt.Errorf("unable to reapply entry '%s': %w", original.GetID().String(), err)
		}

		currentTip, err := r.r.GetReference(rsl.Ref)
		if err != nil {
			return fmt.Errorf("unable to get current tip of the RSL: %w", err)
		}
		replayedIDs[original.GetID().String()] = currentTip
		slog.DebugContext(ctx, fmt.Sprintf("New entry ID for '%s' is '%s'", original.GetID().String(), currentTip.String()))
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

		referenceUpdateDirectives := map[string]githash.Hash{
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

	referenceUpdateDirectives := map[string]githash.Hash{
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

func getRSLEntriesUntil(repo gitstore.Storer, start, until githash.Hash) ([]rsl.Entry, error) {
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

// addUpdatedRefs records every ref that entry updates into refs. It handles
// the same entry types as the replay switch in ReconcileLocalRSLWithRemote, so
// an entry type this build does not know cannot silently contribute no refs
// and slip past the conflict check.
func addUpdatedRefs(refs *set.Set[string], entry rsl.Entry) error {
	switch entry := entry.(type) {
	case *rsl.ReferenceEntry:
		refs.Add(entry.RefName)
	case *rsl.PropagationEntry:
		refs.Add(entry.RefName)
	case *rsl.BulkReferenceEntry:
		for _, update := range entry.Updates {
			refs.Add(update.RefName)
		}
	case *rsl.AnnotationEntry:
		// Annotations refer to prior entries and update no refs of their own.
	default:
		return fmt.Errorf("%w: cannot identify the refs updated by entry type %T", rsl.ErrUnknownRSLEntryType, entry)
	}

	return nil
}

func getLatestRefTipsFromRSLEntries(entries []rsl.Entry) map[string]githash.Hash {
	refTips := map[string]githash.Hash{}
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
		case *rsl.BulkReferenceEntry:
			// The annotations apply to the entry, so they are looked up once
			// rather than per view.
			annotations, hasAnnotations := annotationsMap[entry.GetID().String()]
			for _, view := range entry.ReferenceEntries() {
				if _, has := refTips[view.RefName]; has {
					continue
				}

				if hasAnnotations && view.SkippedBy(annotations) {
					continue
				}

				refTips[view.RefName] = view.TargetID
			}
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
func (r *Repository) isDuplicateEntry(refName string, targetID githash.Hash) (bool, error) {
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

	controllerRepositories := rootMetadata.GetControllerRepositories()
	seenControllerRepositoryLocations := set.NewSet[string]()
	for len(controllerRepositories) != 0 {
		controllerRepository := controllerRepositories[0]
		controllerRepositories = controllerRepositories[1:]

		if seenControllerRepositoryLocations.Has(controllerRepository.GetLocation()) {
			continue
		}

		seenControllerRepositoryLocations.Add(controllerRepository.GetLocation())

		if _, has := upstreamRepositoryDirectivesMapping[controllerRepository.GetLocation()]; !has {
			upstreamRepositoryDirectivesMapping[controllerRepository.GetLocation()] = []tuf.PropagationDirective{}
		}

		// Calculate base64 encoding of the URL: this is the subdirectory where
		// the contents are sent
		encodedLocation := base64.URLEncoding.EncodeToString([]byte(controllerRepository.GetLocation()))

		// FIXME: this assumes tufv01.PropagationDirective
		directive := tufv01.NewPropagationDirective(
			// directive name
			fmt.Sprintf("%s-%s-%s", tuf.GittufControllerPrefix, controllerRepository.GetName(), encodedLocation),
			// upstream location
			controllerRepository.GetLocation(),
			// upstream ref
			policy.PolicyRef,
			// upstream path
			"metadata",
			// downstream ref
			policy.PolicyRef,
			// downstream path
			fmt.Sprintf("%s/%s-%s", tuf.GittufControllerPrefix, controllerRepository.GetName(), encodedLocation),
		)
		upstreamRepositoryDirectivesMapping[controllerRepository.GetLocation()] = append(upstreamRepositoryDirectivesMapping[controllerRepository.GetLocation()], directive)

		// FIXME: we're cloning some repositories twice, once to see the
		// manifest to resolve controller graph, another time to actually
		// propagate contents

		// DFS to resolve transitive propagations
		upstreamRepositoryLocation, err := os.MkdirTemp("", "gittuf-controller-resolve")
		if err != nil {
			return err
		}
		defer os.RemoveAll(upstreamRepositoryLocation) //nolint:errcheck

		fetchReferences := set.NewSetFromItems(rsl.Ref, policy.PolicyRef)

		upstreamRepository, err := gitinterface.CloneAndFetchRepository(controllerRepository.GetLocation(), upstreamRepositoryLocation, "", fetchReferences.Contents(), true)
		if err != nil {
			return fmt.Errorf("unable to fetch controller repository '%s': %w", controllerRepository.GetLocation(), err)
		}

		upstreamState, err := policy.LoadCurrentState(ctx, upstreamRepository, policy.PolicyRef, policyopts.WithInitialRootPrincipals(controllerRepository.GetInitialRootPrincipals()))
		if err != nil {
			return err
		}

		upstreamRootMetadata, err := upstreamState.GetRootMetadata(false)
		if err != nil {
			return err
		}

		upstreamControllerRepositories := upstreamRootMetadata.GetControllerRepositories()
		upstreamControllerRepositories = append(upstreamControllerRepositories, controllerRepositories...)
		controllerRepositories = upstreamControllerRepositories
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

		if err := propagation.PropagateChangesFromUpstreamRepository(r.r, upstreamRepository, directives, sign); err != nil {
			// TODO: atomic? abort?
			return err
		}
	}

	return nil
}
