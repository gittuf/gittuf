// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package policy

import "github.com/gittuf/gittuf/internal/tuf"

type PolicyVerifierOptions struct { //nolint:revive
	InitialRootPrincipals []tuf.Principal
}

type PolicyVerifierOption func(*PolicyVerifierOptions) //nolint:revive

func WithInitialRootPrincipals(initialRootPrincipals []tuf.Principal) PolicyVerifierOption {
	return func(o *PolicyVerifierOptions) {
		o.InitialRootPrincipals = initialRootPrincipals
	}
}
