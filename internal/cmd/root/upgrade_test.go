// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package root

import (
	"errors"
	"fmt"
	"testing"

	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/gittuf/gittuf/pkg/rsl"
	"github.com/stretchr/testify/assert"
)

func TestIsUpgradeRequiredError(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		err      error
		expected bool
	}{
		"nil":                              {err: nil, expected: false},
		"unrelated error":                  {err: errors.New("something else"), expected: false},
		"unknown RSL entry type":           {err: rsl.ErrUnknownRSLEntryType, expected: true},
		"unknown root metadata version":    {err: tuf.ErrUnknownRootMetadataVersion, expected: true},
		"unknown targets metadata version": {err: tuf.ErrUnknownTargetsMetadataVersion, expected: true},
		"wrapped":                          {err: fmt.Errorf("loading RSL: %w", rsl.ErrUnknownRSLEntryType), expected: true},
		"invalid RSL entry is not enough":  {err: rsl.ErrInvalidRSLEntry, expected: false},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.expected, IsUpgradeRequiredError(test.err))
		})
	}
}
