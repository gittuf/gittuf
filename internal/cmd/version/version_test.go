// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package version

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	internalversion "github.com/gittuf/gittuf/internal/version"
)

func TestVersionRun(t *testing.T) {
	oldStdout := os.Stdout

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	cmd := New()
	if err := cmd.Execute(); err != nil {
		w.Close()
		os.Stdout = oldStdout
		t.Fatal(err)
	}

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}
	output := buf.String()

	expected := "gittuf version"
	if !strings.Contains(output, expected) {
		t.Errorf("expected output to contain %q, got %q", expected, output)
	}
}

func TestVersionPrefixV(t *testing.T) {
	internalversion.SetGitVersionForTest("v0.14.0")
	defer internalversion.SetGitVersionForTest("devel")

	oldStdout := os.Stdout

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	cmd := New()
	if err := cmd.Execute(); err != nil {
		w.Close()
		os.Stdout = oldStdout
		t.Fatal(err)
	}

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}
	output := buf.String()

	// Verify that the prefix 'v' was successfully stripped
	expected := "gittuf version 0.14.0"
	if !strings.Contains(output, expected) {
		t.Errorf("expected output to contain %q, got %q", expected, output)
	}
}
