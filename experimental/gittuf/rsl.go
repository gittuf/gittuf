// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gittuf

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"

	rslopts "github.com/gittuf/gittuf/experimental/gittuf/options/rsl"
	"github.com/gittuf/gittuf/internal/common/set"
	"github.com/gittuf/gittuf/internal/dev"
	"github.com/gittuf/gittuf/internal/display"
	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/go-git/go-git/v5/plumbing/transport"
)

const gittufTransportPrefix = "gittuf::"

var (
	ErrCommitNotInRef = errors.New("specified commit is not in ref")
	ErrPushingRSL     = errors.New("unable to push RSL")
	ErrPullingRSL     = errors.New("unable to pull RSL")
)

// RecordRSLEntryForReference is the interface for the user to add an RSL entry
// for the specified Git reference.
func (r *Repository) RecordRSLEntryForReference(refName string, signCommit bool, opts ...rslopts.Option) error {
	if signCommit {
		slog.Debug("Checking if Git signing is configured...")
		err := r.r.CanSign()
		if err != nil {
			return err
		}
	}

	options := &rslopts.Options{}
	for _, fn := range opts {
		fn(options)
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

	slog.Debug("Checking for existing entry for reference with same target...")
	isDuplicate, err := r.isDuplicateEntry(refName, refTip)
	if err != nil {
		return err
	}
	if isDuplicate {
		return nil
	}

	// TODO: once policy verification is in place, the signing key used by
	// signCommit must be verified for the refName in the delegation tree.

	slog.Debug("Creating RSL reference entry...")
	return rsl.NewReferenceEntry(refName, refTip).Commit(r.r, signCommit)
}

// RecordRSLEntryForReferenceAtTarget is a special version of
// RecordRSLEntryForReference used for evaluation. It is only invoked when
// gittuf is explicitly set in developer mode.
func (r *Repository) RecordRSLEntryForReferenceAtTarget(refName, targetID string, signingKeyBytes []byte, opts ...rslopts.Option) error {
	// Double check that gittuf is in developer mode
	if !dev.InDevMode() {
		return dev.ErrNotInDevMode
	}

	options := &rslopts.Options{}
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
func (r *Repository) RecordRSLAnnotation(rslEntryIDs []string, skip bool, message string, signCommit bool) error {
	if signCommit {
		slog.Debug("Checking if Git signing is configured...")
		err := r.r.CanSign()
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
	return rsl.NewAnnotationEntry(rslEntryHashes, skip, message).Commit(r.r, signCommit)
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

// CheckRemoteRSLForUpdates checks if the RSL at the specified remote
// repository has updated in comparison with the local repository's RSL. This is
// done by fetching the remote RSL to the local repository's remote RSL tracker.
// If the remote RSL has been updated, this method also checks if the local and
// remote RSLs have diverged. In summary, the first return value indicates if
// there is an update and the second return value indicates if the two RSLs have
// diverged and need to be reconciled.
//
// Deprecated: this was a precursor to ReconcileLocalRSLWithRemote, we probably
// don't need both of them.
func (r *Repository) CheckRemoteRSLForUpdates(_ context.Context, remoteName string) (bool, bool, error) {
	remoteURL, err := r.r.GetRemoteURL(remoteName)
	if err != nil {
		return false, false, err
	}
	if strings.HasPrefix(remoteURL, gittufTransportPrefix) {
		slog.Debug("Creating new remote to avoid using gittuf transport...")
		remoteName = fmt.Sprintf("check-remote-%s", remoteName)
		if err := r.r.AddRemote(remoteName, strings.TrimPrefix(remoteURL, gittufTransportPrefix)); err != nil {
			return false, false, err
		}
		defer r.r.RemoveRemote(remoteName) //nolint:errcheck
	}

	trackerRef := rsl.RemoteTrackerRef(remoteName)
	rslRemoteRefSpec := []string{fmt.Sprintf("%s:%s", rsl.Ref, trackerRef)}

	slog.Debug(fmt.Sprintf("Updating remote RSL tracker (%s)...", rslRemoteRefSpec))
	if err := r.r.FetchRefSpec(remoteName, rslRemoteRefSpec); err != nil {
		if errors.Is(err, transport.ErrEmptyRemoteRepository) {
			// Check if remote is empty and exit appropriately
			return false, false, nil
		}
		return false, false, err
	}

	remoteRefState, err := r.r.GetReference(trackerRef)
	if err != nil {
		return false, false, err
	}
	slog.Debug(fmt.Sprintf("Remote RSL is at '%s'", remoteRefState.String()))

	localRefState, err := r.r.GetReference(rsl.Ref)
	if err != nil {
		return false, false, err
	}
	slog.Debug(fmt.Sprintf("Local RSL is at '%s'", localRefState.String()))

	// Check if local is nil and exit appropriately
	if localRefState.IsZero() {
		// Local RSL has not been populated but remote is not zero
		// So there are updates the local can pull
		slog.Debug("Local RSL has not been initialized but remote RSL exists")
		return true, false, nil
	}

	// Check if equal and exit early if true
	if remoteRefState.Equal(localRefState) {
		slog.Debug("Local and remote RSLs have same state")
		return false, false, nil
	}

	// Next, check if remote is ahead of local
	knows, err := r.r.KnowsCommit(remoteRefState, localRefState)
	if err != nil {
		return false, false, err
	}
	if knows {
		slog.Debug("Remote RSL is ahead of local RSL")
		return true, false, nil
	}

	// If not ancestor, local may be ahead or they may have diverged
	// If remote is ancestor, only local is ahead, no updates
	// If remote is not ancestor, the two have diverged, local needs to pull updates
	knows, err = r.r.KnowsCommit(localRefState, remoteRefState)
	if err != nil {
		return false, false, err
	}
	if knows {
		slog.Debug("Local RSL is ahead of remote RSL")
		return false, false, nil
	}

	slog.Debug("Local and remote RSLs have diverged")
	return true, true, nil
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
// same target ID Note that it's legal for the RSL to have target A, then B,
// then A again, this is not considered a duplicate entry
func (r *Repository) isDuplicateEntry(refName string, targetID gitinterface.Hash) (bool, error) {
	latestUnskippedEntry, _, err := rsl.GetLatestReferenceEntry(r.r, rsl.ForReference(refName), rsl.IsUnskipped())
	if err != nil {
		if errors.Is(err, rsl.ErrRSLEntryNotFound) {
			return false, nil
		}
		return false, err
	}

	return latestUnskippedEntry.TargetID.Equal(targetID), nil
}

// PrintRSLEntryLog prints a list of all rsl entries to the console, both reading entries and writing entries happens
// in a buffered manner, the buffer size is dictated by the max buffer size
func PrintRSLEntryLog(repo *Repository, bufferedWriter io.WriteCloser, display display.DisplayFunctionHolder) error {
	defer bufferedWriter.Close() //nolint:errcheck

	allReferenceEntries := []*rsl.ReferenceEntry{}
	emptyAnnotationMap := make(map[string][]*rsl.AnnotationEntry)
	annotationMap := make(map[string][]*rsl.AnnotationEntry)

	iteratorEntry, err := rsl.GetLatestEntry(repo.r)
	if err != nil {
		return err
	}

	// Display all reference entries
	for {
		switch iteratorEntry := iteratorEntry.(type) {
		case *rsl.ReferenceEntry:
			allReferenceEntries = append(allReferenceEntries, iteratorEntry)
			if err := display.DisplayLog([]*rsl.ReferenceEntry{iteratorEntry}, emptyAnnotationMap, bufferedWriter); err != nil {
				return nil
			}
		case *rsl.AnnotationEntry:
			for _, targetID := range iteratorEntry.RSLEntryIDs {
				if _, has := annotationMap[targetID.String()]; !has {
					annotationMap[targetID.String()] = []*rsl.AnnotationEntry{}
				}

				annotationMap[targetID.String()] = append(annotationMap[targetID.String()], iteratorEntry)
			}
		}

		parentEntry, err := rsl.GetParentForEntry(repo.r, iteratorEntry)
		if err != nil {
			if errors.Is(err, rsl.ErrRSLEntryNotFound) {
				break
			}

			return err
		}

		iteratorEntry = parentEntry
	}

	// Display all annotation entries

	for index, entry := range allReferenceEntries {
		if index == 0 {
			display.DisplayHeader(bufferedWriter, "Annotations")
		}

		targetID := entry.GetID().String()
		if _, exists := annotationMap[targetID]; exists {
			if err := display.DisplayLog([]*rsl.ReferenceEntry{entry}, annotationMap, bufferedWriter); err != nil {
				return nil
			}

		}
	}

	return nil
}
