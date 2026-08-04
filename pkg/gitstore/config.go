// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitstore

// ConfigKey is the canonical, lowercase name of a Git config setting the store
// can look up. It is an open string type: the constants below document the
// settings gittuf reads today, but any well-formed "section.key" is valid.
type ConfigKey string

const (
	ConfigUserName       ConfigKey = "user.name"
	ConfigUserEmail      ConfigKey = "user.email"
	ConfigUserSigningKey ConfigKey = "user.signingkey"

	ConfigGPGFormat      ConfigKey = "gpg.format"
	ConfigGPGProgram     ConfigKey = "gpg.program"
	ConfigGPGX509Program ConfigKey = "gpg.x509.program"

	ConfigGitsignIssuer      ConfigKey = "gitsign.issuer"
	ConfigGitsignClientID    ConfigKey = "gitsign.clientid"
	ConfigGitsignFulcio      ConfigKey = "gitsign.fulcio"
	ConfigGitsignRekor       ConfigKey = "gitsign.rekor"
	ConfigGitsignRedirectURL ConfigKey = "gitsign.redirecturl"

	ConfigCoreSSHCommand ConfigKey = "core.sshcommand"

	ConfigCacheAutomatic ConfigKey = "gittuf.cache.automatic"
)
