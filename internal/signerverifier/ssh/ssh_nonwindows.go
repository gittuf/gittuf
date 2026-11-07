// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package ssh

// PrepareTestKeyFile is a no-op on non-Windows platforms.
func PrepareTestKeyFile(_ string) error {
	return nil
}
