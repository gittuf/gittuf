// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package verify

type Options struct {
	RefNameOverride             string
	ExportAttestationsDirectory string
	LatestOnly                  bool
}

type Option func(o *Options)

func WithOverrideRefName(refNameOverride string) Option {
	return func(o *Options) {
		o.RefNameOverride = refNameOverride
	}
}

func WithExportAttestationsDirectory(exportAttestationsDirectory string) Option {
	return func(o *Options) {
		o.ExportAttestationsDirectory = exportAttestationsDirectory
	}
}

func WithLatestOnly() Option {
	return func(o *Options) {
		o.LatestOnly = true
	}
}
