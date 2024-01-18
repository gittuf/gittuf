// SPDX-License-Identifier: Apache-2.0

package featureflags

// UseGitBinary allows the user to indicate the Git binary must be used instead
// of go-git for interactions with the underlying repository where both are
// supported.
var UseGitBinary = false

// UsePolicyPathCache allows the user to indicate that during verification, the
// verifiers identified for some path must be cached for the policy state.
var UsePolicyPathCache = false
