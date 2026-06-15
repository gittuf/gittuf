// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package version

import (
	"testing"

	"github.com/gittuf/gittuf/internal/cmd"
	"github.com/stretchr/testify/assert"
)

func TestVersion(t *testing.T) {
	t.Run("run", func(t *testing.T) {
		_, stdOut, _, err := cmd.ExecuteCommandC(New(), "version")
		if err != nil {
			t.Fatal(err)
		}

		expected := "gittuf version"
		assert.Contains(t, stdOut.String(), expected)
	})
}
