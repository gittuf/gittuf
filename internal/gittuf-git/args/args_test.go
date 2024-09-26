// SPDX-License-Identifier: Apache-2.0

package args

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProcessArgs(t *testing.T) {
	// TODO: Test this more
	tests := map[string]struct {
		args     []string
		expected Args
	}{
		"no arguments": {
			args: []string{},
			expected: Args{
				GlobalFlags: nil,
				Command:     "",
				Parameters:  nil,
			},
		},
		"pull": {
			args: []string{"pull"},
			expected: Args{
				GlobalFlags: []string{},
				Command:     "pull",
				Parameters:  []string{},
			},
		},
		"push": {
			args: []string{"push"},
			expected: Args{
				GlobalFlags: []string{},
				Command:     "push",
				Parameters:  []string{},
			},
		},
		"fetch origin": {
			args: []string{"fetch", "origin"},
			expected: Args{
				GlobalFlags: []string{},
				Command:     "fetch",
				Parameters:  []string{"origin"},
			},
		},
		"-C ../somedir fetch origin": {
			args: []string{"-C", "../somedir", "fetch", "origin"},
			expected: Args{
				GlobalFlags: []string{"-C", "../somedir"},
				Command:     "fetch",
				Parameters:  []string{"origin"},
				ChdirIdx:    1,
			},
		},
	}

	for name, test := range tests {
		got := ProcessArgs(test.args)
		assert.Equal(t, test.expected, got, fmt.Sprintf("unexpected result in test '%s'", name))
	}
}

func TestLocateCommand(t *testing.T) {
	// TODO: Test this more
	tests := map[string]struct {
		args             []string
		expectedCmdIdx   int
		expectedCfgIdx   int
		expectedChdirIdx int
	}{
		"no arguments": {
			args:             []string{},
			expectedCmdIdx:   0,
			expectedCfgIdx:   0,
			expectedChdirIdx: 0,
		},
		"pull": {
			args:             []string{"pull"},
			expectedCmdIdx:   0,
			expectedCfgIdx:   0,
			expectedChdirIdx: 0,
		},
		"push": {
			args:             []string{"push"},
			expectedCmdIdx:   0,
			expectedCfgIdx:   0,
			expectedChdirIdx: 0,
		},
		"fetch origin": {
			args:             []string{"fetch", "origin"},
			expectedCmdIdx:   0,
			expectedCfgIdx:   0,
			expectedChdirIdx: 0,
		},
		"-C ../somedir fetch origin": {
			args:             []string{"-C", "../somedir", "fetch", "origin"},
			expectedCmdIdx:   2,
			expectedCfgIdx:   0,
			expectedChdirIdx: 1,
		},
	}

	for name, test := range tests {
		cmdIdx, cfgIdx, chdirIdx := locateCommand(test.args)

		assert.Equal(t, test.expectedCmdIdx, cmdIdx, fmt.Sprintf("unexpected result in test '%s'", name))
		assert.Equal(t, test.expectedCfgIdx, cfgIdx, fmt.Sprintf("unexpected result in test '%s'", name))
		assert.Equal(t, test.expectedChdirIdx, chdirIdx, fmt.Sprintf("unexpected result in test '%s'", name))
	}
}

func TestDetermineRemote(t *testing.T) {
	// TODO: Test this more
	tests := map[string]struct {
		args     []string
		expected string
	}{
		"simple case": {
			args:     []string{"origin"},
			expected: "origin",
		},
	}

	for name, test := range tests {
		remote := DetermineRemote(test.args)

		assert.Equal(t, test.expected, remote, fmt.Sprintf("unexpected result in test '%s'", name))
	}
}
