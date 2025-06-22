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
	return "cat"
}

func (p *pagerTestCat) getFlags() []string {
	return nil
}

func getPagerTestNone() pager {
	return nil
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
