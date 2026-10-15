// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package version

import (
	"io"
	"testing"

	"github.com/gittuf/gittuf/internal/cmd"
	"github.com/stretchr/testify/assert"
)

type failingWriter struct {
	writesBeforeFailure int
}

func (w *failingWriter) Write(p []byte) (int, error) {
	if w.writesBeforeFailure == 0 {
		return 0, io.ErrClosedPipe
	}
	w.writesBeforeFailure--
	return len(p), nil
}

func TestVersionOutputErrors(t *testing.T) {
	for _, test := range []struct {
		name                string
		devMode             string
		writesBeforeFailure int
	}{
		{name: "version output", devMode: "0"},
		{name: "developer mode output", devMode: "1", writesBeforeFailure: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("GITTUF_DEV", test.devMode)
			command := New()
			command.SetOut(&failingWriter{writesBeforeFailure: test.writesBeforeFailure})
			command.SetErr(io.Discard)
			command.SetArgs([]string{})

			assert.ErrorIs(t, command.Execute(), io.ErrClosedPipe)
		})
	}
}

func TestVersion(t *testing.T) {
	t.Run("default (no dev mode env var)", func(t *testing.T) {
		t.Setenv("GITTUF_DEV", "")

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
