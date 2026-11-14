// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package ssh

func prepareKeyFileForSigning(_ string) error {
	return nil
}
