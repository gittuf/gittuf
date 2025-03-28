// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package hooks

import (
	"errors"

	"github.com/gittuf/gittuf/internal/tuf"
)

var (
	ErrInvalidHookExecutionAttestation = errors.New("hook execution attestation does not match expected details")
)

// HookExecutionAttestation records the results of hook execution, and
type HookExecutionAttestation interface {
	// GetTargetRef returns the name of the reference on which the hooks were
	// run.
	GetTargetRef() string

	// GetTargetID returns the Git ID of the reference on which the hooks were
	// run.
	GetTargetID() string

	// GetPolicyEntry returns the GitID of the policy entry from which the
	// hooks were loaded from.
	GetPolicyEntry() string

	// GetHookStage returns the name of the stage on which the hooks were run.
	GetHookStage() tuf.HookStage

	// GetHooks returns the names of the hooks which were run successfully.
	GetHooks() []string

	// GetHookRunner returns the ID of the principal who ran the hook.
	GetHookRunner() string
}
