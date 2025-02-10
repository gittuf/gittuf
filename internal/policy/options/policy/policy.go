// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package policy

import "github.com/gittuf/gittuf/internal/tuf"

type LoadStateOptions struct {
	InitialRootPrincipals []tuf.Principal
}

type LoadStateOption func(*LoadStateOptions)

func WithInitialRootPrincipals(initialRootPrincipals []tuf.Principal) LoadStateOption {
	return func(o *LoadStateOptions) {
		o.InitialRootPrincipals = initialRootPrincipals
	}
}
