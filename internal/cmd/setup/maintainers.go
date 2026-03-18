// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/policy"
)

// checkRootExists returns true if gittuf has already been initialized.
func checkRootExists(repo *gittuf.Repository) bool {
	_, err := repo.GetGitRepository().GetReference(policy.PolicyRef)
	return err == nil
}
