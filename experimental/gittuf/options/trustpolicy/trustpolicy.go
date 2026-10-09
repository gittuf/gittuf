// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package trustpolicy

type Options struct {
	CreateRSLEntry    bool
	InheritPrincipals bool
}

type Option func(o *Options)

func WithRSLEntry() Option {
	return func(o *Options) {
		o.CreateRSLEntry = true
	}
}

func WithInheritPrincipals() Option {
	return func(o *Options) {
		o.InheritPrincipals = true
	}
}
