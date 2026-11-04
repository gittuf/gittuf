// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package display

import (
	"bytes"
	"fmt"
	"io"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type pagerTestCat struct{}

func getPagerTestCat() pager {
	return &pagerTestCat{}
}

func (p *pagerTestCat) getBinary() string {
	if runtime.GOOS == "windows" {
		return "findstr"
	}
	return "cat"
}

func (p *pagerTestCat) getFlags() []string {
	if runtime.GOOS == "windows" {
		return []string{"/R", "^"}
	}
	return nil
}

func getPagerTestNone() pager {
	return nil
}

func TestPagerResolution(t *testing.T) {
	// Ensure results are deterministic.
	t.Setenv("PAGER", "")
	t.Setenv("LESS", "")

	pagerVar := newPagerEnvVar()
	assert.Equal(t, &pagerEnvVar{}, pagerVar)
	t.Setenv("PAGER", "less")

	pager := getPagerReal()
	assert.Equal(t, "less", pager.getBinary())
	assert.Equal(t, []string{}, pager.getFlags())

	pager = newPagerLess()
	assert.Equal(t, "less", pager.getBinary())
	assert.Equal(t, []string{"-F", "-R", "-X"}, pager.getFlags())

	t.Setenv("LESS", "A")
	assert.Equal(t, []string{"-A"}, pager.getFlags())

	pager = newPagerMore()
	assert.Equal(t, "more", pager.getBinary())
	assert.Len(t, pager.getFlags(), 0)
}

func TestNewDisplayWriter(t *testing.T) {
	tests := map[string]struct {
		contents []byte
		page     bool
	}{
		"without paging": {
			contents: []byte("Hello, world!"),
			page:     false,
		},
		"with paging": {
			contents: []byte("Hello, world!"),
			page:     true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if test.page {
				getPager = getPagerTestCat
			} else {
				getPager = getPagerTestNone
			}

			output := &bytes.Buffer{}
			writer := NewDisplayWriter(output)

			_, err := writer.Write(test.contents)
			if err != nil {
				t.Fatal(err)
			}

			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}

			gotOutput := output.String()
			if runtime.GOOS == "windows" {
				gotOutput = strings.TrimSpace(gotOutput)
			}
			assert.Equal(t, string(test.contents), gotOutput, fmt.Sprintf("unexpected result in test '%s', got '%s', want '%s'", name, gotOutput, string(test.contents)))
		})
	}
}

type mockWriteCloser struct {
	io.Writer
}

func (m mockWriteCloser) Close() error {
	return nil
}

func TestNewDisplayWriter_WriteCloser(t *testing.T) {
	getPager = getPagerTestNone
	output := &mockWriteCloser{Writer: &bytes.Buffer{}}
	writer := NewDisplayWriter(output)
	assert.Equal(t, output, writer)
}

func TestGetPagerReal_NotFound(t *testing.T) {
	t.Setenv("PATH", "")
	t.Setenv("PAGER", "nonexistent-pager")
	pager := getPagerReal()
	assert.Nil(t, pager)
}

type pagerTestInvalid struct{}

func getPagerTestInvalid() pager               { return &pagerTestInvalid{} }
func (p *pagerTestInvalid) getBinary() string  { return "nonexistent-pager-binary" }
func (p *pagerTestInvalid) getFlags() []string { return nil }

func TestPagerWriteCloser_StartError(t *testing.T) {
	getPager = getPagerTestInvalid
	output := &bytes.Buffer{}
	writer := NewDisplayWriter(output)

	_, err := writer.Write([]byte("test"))
	assert.Error(t, err)

	// Close should also handle not started
	err = writer.Close()
	if err != nil {
		// exec.Cmd.Start closes the pipe on failure, so this might return an error
		t.Logf("Close returned error: %v", err)
	}
}
func TestPagerWriteCloser_StdinPipeError(t *testing.T) {
	getPager = getPagerTestCat
	output := &bytes.Buffer{}
	writer := NewDisplayWriter(output)

	pagerWriter := writer.(*pagerWriteCloser)
	_, _ = pagerWriter.command.StdinPipe() // call once

	_, err := writer.Write([]byte("test"))
	assert.Error(t, err)
}

func TestPagerWriteCloser_WaitError(t *testing.T) {
	getPager = getPagerTestCat
	output := &bytes.Buffer{}
	writer := NewDisplayWriter(output)

	// Start it normally
	_, err := writer.Write([]byte("test"))
	assert.NoError(t, err)

	pagerWriter := writer.(*pagerWriteCloser)
	// kill the process so Wait returns an error
	_ = pagerWriter.command.Process.Kill()

	err = writer.Close()
	assert.Error(t, err)
}
