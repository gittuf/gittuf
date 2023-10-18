// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"errors"

	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/rsl"
)

var ErrPushingPolicy = errors.New("unable to push policy")

// PushPolicy pushes the local gittuf policy to the specified remote. As this
// push defaults to fast-forward only, divergent policy states are detected.
// Note that this also pushes the RSL as the policy cannot change without an
// update to the RSL.
func (r *Repository) PushPolicy(ctx context.Context, remoteName string) error {
	if err := gitinterface.Push(ctx, r.r, remoteName, []string{policy.PolicyRef, rsl.Ref}); err != nil {
		return errors.Join(ErrPushingPolicy, err)
	}

	return nil
}
