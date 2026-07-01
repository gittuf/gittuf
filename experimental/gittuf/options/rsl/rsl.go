// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package rsl

type RecordOptions struct {
	RefNameOverride       string
	RemoteName            string
	LocalOnly             bool
	SkipCheckForDuplicate bool
	SigningKeyBytes       []byte
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

// WithRecordSigningKeyBytes provides a PEM-encoded private key to sign the RSL
// entry commit with directly, instead of reading user.signingKey from git
// config. It is only consulted when RecordRSLEntryForReference is called with
// signCommit=true; with signCommit=false the entry is unsigned regardless.
// Intended for embedders (forges) that hold their own signing key and operate
// on bare repositories with no per-repo git config.
func WithRecordSigningKeyBytes(pem []byte) RecordOption {
	return func(o *RecordOptions) {
		o.SigningKeyBytes = pem
	}
}

type AnnotateOptions struct {
	RemoteName      string
	LocalOnly       bool
	SigningKeyBytes []byte
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

// WithAnnotateSigningKeyBytes provides a PEM-encoded private key to sign the
// annotation commit with directly. See WithRecordSigningKeyBytes.
func WithAnnotateSigningKeyBytes(pem []byte) AnnotateOption {
	return func(o *AnnotateOptions) {
		o.SigningKeyBytes = pem
	}
}
