// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package version

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
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

func TestVersionDevModeInactive(t *testing.T) {
	t.Setenv("GITTUF_DEV", "0")

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

	expectedWarning := "gittuf is operating in developer mode"
	if strings.Contains(output, expectedWarning) {
		t.Errorf("expected output NOT to contain %q, got %q", expectedWarning, output)
	}
}
