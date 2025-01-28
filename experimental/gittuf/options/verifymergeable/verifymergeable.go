// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package verifymergeable

type Options struct {
	BypassRSLForFeatureRef bool
}

type Option func(o *Options)

func WithBypassRSLForFeatureRef() Option {
	return func(o *Options) {
		o.BypassRSLForFeatureRef = true
	}
}
