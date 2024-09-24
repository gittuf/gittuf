// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package authorizations

type ReferenceAuthorization interface {
	GetRef() string
	GetFromID() string
	GetTargetID() string
}
