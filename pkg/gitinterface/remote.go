// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

// AddRemote adds a remote with the specified name and URL.
func (r *Repository) AddRemote(remoteName, url string) error {
	_, err := r.executor("remote", "add", remoteName, url).executeString()
	return err
}

// RemoveRemote removes the remote with the specified name.
func (r *Repository) RemoveRemote(remoteName string) error {
	_, err := r.executor("remote", "remove", remoteName).executeString()
	return err
}

// GetRemoteURL gets the URL of the remote with the specified name.
func (r *Repository) SetRemote(remoteName string, url string) error {
	_, err := r.executor("remote", "set-url", remoteName, url).executeString()
	return err
}

func (r *Repository) GetRemoteURL(remoteName string) (string, error) {
	return r.executor("remote", "get-url", remoteName).executeString()
}
