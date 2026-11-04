// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package display

import (
	"bytes"
	"fmt"
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

func TestPagerEnvVarWhitespace(t *testing.T) {
	tests := map[string]struct {
		env    string
		binary string
		flags  []string
	}{
		"single spaces":     {env: "less -R", binary: "less", flags: []string{"-R"}},
		"repeated spaces":   {env: "less  -R   -X", binary: "less", flags: []string{"-R", "-X"}},
		"tab separated":     {env: "less\t-R", binary: "less", flags: []string{"-R"}},
		"surrounding space": {env: "  less -R  ", binary: "less", flags: []string{"-R"}},
		"only whitespace":   {env: " \t ", binary: "", flags: nil},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Setenv("PAGER", test.env)

			pager := newPagerEnvVar()
			assert.Equal(t, test.binary, pager.getBinary())
			assert.Equal(t, test.flags, pager.getFlags())
		})
	}
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

type bufferWriteCloser struct {
	bytes.Buffer
	closed bool
}

func (b *bufferWriteCloser) Close() error {
	b.closed = true
	return nil
}

func TestNewDisplayWriterWithoutPagerUsesWriteCloser(t *testing.T) {
	getPager = getPagerTestNone
	defer func() { getPager = getPagerReal }()

	output := &bufferWriteCloser{}
	writer := NewDisplayWriter(output)

	// An output that's already an io.WriteCloser is returned as is
	assert.Same(t, output, writer)

	_, err := writer.Write([]byte("Hello, world!"))
	assert.Nil(t, err)
	assert.Nil(t, writer.Close())
	assert.Equal(t, "Hello, world!", output.String())
	assert.True(t, output.closed)
}
