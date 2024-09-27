// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

func (r *Repository) AddRemote(remoteName, url string) error {
	_, err := r.executor("remote", "add", remoteName, url).executeString()
	return err
}

func (r *Repository) RemoveRemote(remoteName string) error {
	_, err := r.executor("remote", "remove", remoteName).executeString()
	return err
}

func (r *Repository) GetRemoteURL(remoteName string) (string, error) {
	return r.executor("remote", "get-url", remoteName).executeString()
}
