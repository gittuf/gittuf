// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

// HasObject returns true if an object with the specified Git ID exists in the
// repository.
func (r *Repository) HasObject(objectID Hash) bool {
	_, err := r.executor("cat-file", "-e", objectID.String()).executeString()
	return err == nil
}
