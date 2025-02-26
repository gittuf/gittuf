// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package rsl

type RecordOptions struct {
	RefNameOverride       string
	RemoteName            string
	LocalOnly             bool
	SkipCheckForDuplicate bool
}

type RecordOption func(o *RecordOptions)

func WithOverrideRefName(refNameOverride string) RecordOption {
	return func(o *RecordOptions) {
		o.RefNameOverride = refNameOverride
	}
}

// WithSkipCheckForDuplicateEntry indicates that the RSL entry creation must not
// check if the latest entry for the reference has the same target ID.
func WithSkipCheckForDuplicateEntry() RecordOption {
	return func(o *RecordOptions) {
		o.SkipCheckForDuplicate = true
	}
}

func WithRecordRemote(remoteName string) RecordOption {
	return func(o *RecordOptions) {
		o.RemoteName = remoteName
	}
}

func WithRecordLocalOnly() RecordOption {
	return func(o *RecordOptions) {
		o.LocalOnly = true
	}
}

type AnnotateOptions struct {
	RemoteName string
	LocalOnly  bool
}

type AnnotateOption func(o *AnnotateOptions)

func WithAnnotateRemote(remoteName string) AnnotateOption {
	return func(o *AnnotateOptions) {
		o.RemoteName = remoteName
	}
}

func WithAnnotateLocalOnly() AnnotateOption {
	return func(o *AnnotateOptions) {
		o.LocalOnly = true
	}
}
