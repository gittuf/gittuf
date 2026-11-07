// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package ssh

import (
	"os"
	"os/exec"
	"os/user"
	"strings"
)

// prepareKeyFileForSigning ensures that the private key file on Windows
// has strict NTFS permissions (restricted to the current user) as required
// by OpenSSH ssh-keygen. OpenSSH on Windows rejects private keys if
// permissions allow other users (such as 'NT AUTHORITY\Authenticated Users').
func prepareKeyFileForSigning(path string) error {
	// Prefer user's SID to avoid ambiguity between local and domain accounts
	// and eliminate name-to-SID resolution in icacls. Numerical SIDs require a '*' prefix.
	var identity string
	if currentUser, err := user.Current(); err == nil && currentUser.Uid != "" {
		identity = "*" + currentUser.Uid
	} else if username, ok := os.LookupEnv("USERNAME"); ok && username != "" {
		identity = username
	}
	if identity == "" {
		return nil
	}

	// If path points to a public key (<key>.pub), OpenSSH expects the
	// private key at <key>. Ensure permissions are set for both.
	privKeyPath := strings.TrimSuffix(path, ".pub")
	if privKeyPath != path {
		_ = setRestrictedPermissions(privKeyPath, identity)
	}
	return setRestrictedPermissions(path, identity)
}

func setRestrictedPermissions(path, identity string) error {
	if _, err := os.Stat(path); err != nil {
		return nil
	}
	// /inheritance:r removes inherited permissions
	// /grant:r grants access only to the current user or SID
	cmd := exec.Command("icacls", path, "/inheritance:r", "/grant:r", identity+":F") //nolint:gosec
	return cmd.Run()
}
