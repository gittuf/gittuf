// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package github

import (
	"github.com/gittuf/gittuf/internal/attestations/authorizations"
	"github.com/gittuf/gittuf/internal/tuf"
)

type PullRequestApprovalAttestation interface {
	GetApprovers() []*tuf.Key
	GetDismissedApprovers() []*tuf.Key
	authorizations.ReferenceAuthorization
}
