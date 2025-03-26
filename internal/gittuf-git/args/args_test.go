// SPDX-License-Identifier: Apache-2.0

package args

import (
	"fmt"
	"testing"

	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/stretchr/testify/assert"
)

func TestProcessArgs(t *testing.T) {
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
				GitDir:      ".git",
				RootDir:     ".",
			},
		},
		"push": {
			args: []string{"push"},
			expected: Args{
				GlobalFlags: []string{},
				Command:     "push",
				Parameters:  []string{},
				GitDir:      ".git",
				RootDir:     ".",
			},
		},
		"fetch origin": {
			args: []string{"fetch", "origin"},
			expected: Args{
				GlobalFlags: []string{},
				Command:     "fetch",
				Parameters:  []string{"origin"},
				GitDir:      ".git",
				RootDir:     ".",
			},
		},
		"-C ../somedir fetch origin": {
			args: []string{"-C", "../somedir", "fetch", "origin"},
			expected: Args{
				GlobalFlags: []string{"-C", "../somedir"},
				Command:     "fetch",
				Parameters:  []string{"origin"},
				ChdirIdx:    1,
				GitDir:      "../somedir/.git",
				RootDir:     "../somedir",
			},
		},
		"-c core.editor=vim commit": {
			args: []string{"-c", "core.editor=vim", "commit"},
			expected: Args{
				GlobalFlags: []string{"-c", "core.editor=vim"},
				Command:     "commit",
				Parameters:  []string{},
				ChdirIdx:    0,
				ConfigIdx:   1,
				GitDir:      ".git",
				RootDir:     ".",
			},
		},
		"-c user.name=Test -C ../somedir --version": {
			args: []string{"-c", "user.name=Test", "-C", "../somedir", "--version"},
			expected: Args{
				GlobalFlags: []string{"-c", "user.name=Test", "-C", "../somedir"},
				Command:     "--version",
				Parameters:  []string{},
				ChdirIdx:    3,
				ConfigIdx:   1,
				GitDir:      "../somedir/.git",
				RootDir:     "../somedir",
			},
		},
		"-c user.name=Test -C /tmp/git-repo push origin main": {
			args: []string{"-c", "user.name=Test", "-C", "/tmp/git-repo", "push", "origin", "main"},
			expected: Args{
				GlobalFlags: []string{"-c", "user.name=Test", "-C", "/tmp/git-repo"},
				Command:     "push",
				Parameters:  []string{"origin", "main"},
				ChdirIdx:    3,
				ConfigIdx:   1,
				GitDir:      "/tmp/git-repo/.git",
				RootDir:     "/tmp/git-repo",
			},
		},
		"--help": {
			args: []string{"--help"},
			expected: Args{
				GlobalFlags: []string{},
				Command:     "--help",
				Parameters:  []string{},
				ChdirIdx:    0,
				ConfigIdx:   0,
				GitDir:      ".git",
				RootDir:     ".",
			},
		},
		"--noop status": {
			args: []string{"--noop", "status"},
			expected: Args{
				GlobalFlags: []string{"--noop"},
				Command:     "status",
				Parameters:  []string{},
				ChdirIdx:    0,
				ConfigIdx:   0,
				GitDir:      ".git",
				RootDir:     ".",
			},
		},
		"--list-cmds=main": {
			args: []string{"--list-cmds=main"},
			expected: Args{
				GlobalFlags: []string{},
				Command:     "--list-cmds=main",
				Parameters:  []string{},
				ChdirIdx:    0,
				ConfigIdx:   0,
				GitDir:      ".git",
				RootDir:     ".",
			},
		},
	}

	for name, test := range tests {
		got := ProcessArgs(test.args)
		assert.Equal(t, test.expected, got, fmt.Sprintf("unexpected result in test '%s'", name))
	}
}

func TestLocateCommand(t *testing.T) {
	tests := map[string]struct {
		args              []string
		expectedCmdIdx    int
		expectedCfgIdx    int
		expectedChdirIdx  int
		expectedgitDirIdx int
	}{
		"no arguments": {
			args:              []string{},
			expectedCmdIdx:    0,
			expectedCfgIdx:    0,
			expectedChdirIdx:  0,
			expectedgitDirIdx: 0,
		},
		"pull": {
			args:              []string{"pull"},
			expectedCmdIdx:    0,
			expectedCfgIdx:    0,
			expectedChdirIdx:  0,
			expectedgitDirIdx: 0,
		},
		"push": {
			args:              []string{"push"},
			expectedCmdIdx:    0,
			expectedCfgIdx:    0,
			expectedChdirIdx:  0,
			expectedgitDirIdx: 0,
		},
		"fetch origin": {
			args:              []string{"fetch", "origin"},
			expectedCmdIdx:    0,
			expectedCfgIdx:    0,
			expectedChdirIdx:  0,
			expectedgitDirIdx: 0,
		},
		"-C ../somedir fetch origin": {
			args:              []string{"-C", "../somedir", "fetch", "origin"},
			expectedCmdIdx:    2,
			expectedCfgIdx:    0,
			expectedChdirIdx:  1,
			expectedgitDirIdx: 0,
		},
		"-c core.editor=vim commit": {
			args:              []string{"-c", "core.editor=vim", "commit"},
			expectedCmdIdx:    2,
			expectedCfgIdx:    1,
			expectedChdirIdx:  0,
			expectedgitDirIdx: 0,
		},
		"-c user.name=Test -C ../somedir --version": {
			args:              []string{"-c", "user.name=Test", "-C", "../somedir", "--version"},
			expectedCmdIdx:    4,
			expectedCfgIdx:    1,
			expectedChdirIdx:  3,
			expectedgitDirIdx: 0,
		},
		"--help": {
			args:              []string{"--help"},
			expectedCmdIdx:    0,
			expectedCfgIdx:    0,
			expectedChdirIdx:  0,
			expectedgitDirIdx: 0,
		},
		"--noop status": {
			args:              []string{"--noop", "status"},
			expectedCmdIdx:    1,
			expectedCfgIdx:    0,
			expectedChdirIdx:  0,
			expectedgitDirIdx: 0,
		},
		"--list-cmds=main": {
			args:              []string{"--list-cmds=main"},
			expectedCmdIdx:    0,
			expectedCfgIdx:    0,
			expectedChdirIdx:  0,
			expectedgitDirIdx: 0,
		},
		"-c user.name=Test -C /tmp/git-repo push origin main": {
			args:              []string{"-c", "user.name=Test", "-C", "/tmp/git-repo", "push", "origin", "main"},
			expectedCmdIdx:    4,
			expectedCfgIdx:    1,
			expectedChdirIdx:  3,
			expectedgitDirIdx: 0,
		},
	}

	for name, test := range tests {
		cmdIdx, cfgIdx, chdirIdx, girDirIndex := locateCommand(test.args)

		assert.Equal(t, test.expectedCmdIdx, cmdIdx, fmt.Sprintf("unexpected result in test '%s'", name))
		assert.Equal(t, test.expectedCfgIdx, cfgIdx, fmt.Sprintf("unexpected result in test '%s'", name))
		assert.Equal(t, test.expectedChdirIdx, chdirIdx, fmt.Sprintf("unexpected result in test '%s'", name))
		assert.Equal(t, test.expectedgitDirIdx, girDirIndex, fmt.Sprintf("unexpected result in test '%s'", name))
	}
}

func TestDetermineRemote(t *testing.T) {
	gitDir := t.TempDir()
	// TODO: Possibly implement a independent helper here?
	repo := gitinterface.CreateTestGitRepository(t, gitDir, true)
	err := repo.SetGitConfig("remote.notorigin.url", "https://github.com/test/test.git")
	if err != nil {
		t.Fatalf("failed to set git config: %s", err)
	}

	tests := map[string]struct {
		args     []string
		expected string
	}{
		"simple case": {
			args:     []string{"notorigin"},
			expected: "notorigin",
		},
		"remote with colon": {
			args:     []string{"notorigin:main"},
			expected: "notorigin",
		},
		"remote with branch": {
			args:     []string{"notorigin", "main"},
			expected: "notorigin",
		},
		"remote with multiple args": {
			args:     []string{"-f", "notorigin", "main"},
			expected: "notorigin",
		},
		"no args": {
			args:     []string{},
			expected: "notorigin",
		},
		"args without remote": {
			args:     []string{"main"},
			expected: "notorigin",
		},
	}

	for name, test := range tests {
		remote, err := DetermineRemote(test.args, gitDir)
		if err != nil {
			t.Fatalf("unexpected error in test '%s': %s", name, err)
		}

		assert.Equal(t, test.expected, remote, fmt.Sprintf("unexpected result in test '%s'", name))
	}
}
