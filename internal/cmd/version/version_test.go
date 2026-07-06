// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package version

import (
	"testing"

	"github.com/gittuf/gittuf/internal/cmd"
	"github.com/stretchr/testify/assert"
)

func TestVersion(t *testing.T) {
	t.Run("default (no dev mode env var)", func(t *testing.T) {
		_, stdOut, _, err := cmd.ExecuteCommandC(New())
		assert.NoError(t, err)

		output := stdOut.String()
		assert.Contains(t, output, "gittuf version")
		assert.NotContains(t, output, "gittuf is operating in developer mode")
	})

	t.Run("dev mode active", func(t *testing.T) {
		t.Setenv("GITTUF_DEV", "1")

		_, stdOut, _, err := cmd.ExecuteCommandC(New())
		assert.NoError(t, err)

		output := stdOut.String()
		assert.Contains(t, output, "gittuf version")
		assert.Contains(t, output, "gittuf is operating in developer mode")
	})
}
