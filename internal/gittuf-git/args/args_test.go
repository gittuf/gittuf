// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package args

import (
	"fmt"
	"testing"

	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
				GitDir:      ".git",
				RootDir:     ".",
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
	repo := gitinterface.CreateTestGitRepository(t, gitDir, true)
	err := repo.SetGitConfig("remote.notorigin.url", "https://github.com/test/test.git")
	if err != nil {
		t.Fatalf("failed to set git config: %s", err)
	}

	tests := map[string]struct {
		args         Args
		expectedName string
		expectedURL  string
	}{
		"no remote specified": {
			args:         Args{Parameters: []string{""}, RootDir: gitDir},
			expectedName: "notorigin",
			expectedURL:  "https://github.com/test/test.git",
		},
		"just remote specified": {
			args:         Args{Parameters: []string{"notorigin"}, RootDir: gitDir},
			expectedName: "notorigin",
			expectedURL:  "https://github.com/test/test.git",
		},
		"remote with colon": {
			args:         Args{Parameters: []string{"notorigin:main"}, RootDir: gitDir},
			expectedName: "notorigin",
			expectedURL:  "https://github.com/test/test.git",
		},
		"remote with branch": {
			args:         Args{Parameters: []string{"notorigin", "main"}, RootDir: gitDir},
			expectedName: "notorigin",
			expectedURL:  "https://github.com/test/test.git",
		},
		"remote with multiple args": {
			args:         Args{Parameters: []string{"-f", "notorigin", "main"}, RootDir: gitDir},
			expectedName: "notorigin",
			expectedURL:  "https://github.com/test/test.git",
		},
		"no args": {
			args:         Args{Parameters: []string{}, RootDir: gitDir},
			expectedName: "notorigin",
			expectedURL:  "https://github.com/test/test.git",
		},
		"args without remote": {
			args:         Args{Parameters: []string{"main"}, RootDir: gitDir},
			expectedName: "notorigin",
			expectedURL:  "https://github.com/test/test.git",
		},
		"custom remote": {
			args:         Args{Parameters: []string{"git@example.com:/owner/repo", "main"}, RootDir: gitDir},
			expectedName: "git@example.com:/owner/repo",
			expectedURL:  "git@example.com:/owner/repo",
		},
	}

	for name, test := range tests {
		remoteName, remoteURL, err := DetermineRemote(test.args)
		if err != nil {
			t.Fatalf("unexpected error in test '%s': %s", name, err)
		}

		assert.Equal(t, test.expectedName, remoteName, fmt.Sprintf("unexpected result in test '%s'", name))
		assert.Equal(t, test.expectedURL, remoteURL, fmt.Sprintf("unexpected result in test '%s'", name))
	}
}

func TestDeterminePushedRefs(t *testing.T) {
	gitDir := t.TempDir()
	repo := gitinterface.CreateTestGitRepository(t, gitDir, true)

	err := repo.SetGitConfig("remote.origin.url", "https://github.com/test/test.git")
	if err != nil {
		t.Fatalf("failed to set git config: %s", err)
	}

	mainRefName := "refs/heads/main"
	featureRefName := "refs/heads/feature"
	aRefName := "refs/heads/a"
	bRefName := "refs/heads/b"

	treeBuilder := gitinterface.NewTreeBuilder(repo)

	// Write empty tree
	emptyTreeID, err := treeBuilder.WriteTreeFromEntries(nil)
	if err != nil {
		t.Fatal(err)
	}

	_, err = repo.Commit(emptyTreeID, mainRefName, "Initial commit\n", false)
	require.Nil(t, err)
	_, err = repo.Commit(emptyTreeID, featureRefName, "Initial commit\n", false)
	require.Nil(t, err)
	_, err = repo.Commit(emptyTreeID, aRefName, "Initial commit\n", false)
	require.Nil(t, err)
	_, err = repo.Commit(emptyTreeID, bRefName, "Initial commit\n", false)
	require.Nil(t, err)
	err = repo.SetSymbolicReference("HEAD", mainRefName)
	require.Nil(t, err)

	tests := map[string]struct {
		args             Args
		expectedRefSpecs []string
	}{
		"no refs specified": {
			args:             Args{Parameters: []string{}, RootDir: gitDir},
			expectedRefSpecs: []string{"refs/heads/main:refs/heads/main"},
		},
		"no refs specified, remote specified": {
			args:             Args{Parameters: []string{"origin"}, RootDir: gitDir},
			expectedRefSpecs: []string{"refs/heads/main:refs/heads/main"},
		},
		"one ref specified": {
			args:             Args{Parameters: []string{"main"}, RootDir: gitDir},
			expectedRefSpecs: []string{"refs/heads/main:refs/heads/main"},
		},
		"two refs specified": {
			args:             Args{Parameters: []string{"main", "feature"}, RootDir: gitDir},
			expectedRefSpecs: []string{"refs/heads/main:refs/heads/main", "refs/heads/feature:refs/heads/feature"},
		},
		"one refspec specified": {
			args:             Args{Parameters: []string{"refs/heads/main:refs/heads/main"}, RootDir: gitDir},
			expectedRefSpecs: []string{"refs/heads/main:refs/heads/main"},
		},
		"two refspecs specified": {
			args:             Args{Parameters: []string{"refs/heads/main:refs/heads/main", "refs/heads/feature:refs/heads/feature"}, RootDir: gitDir},
			expectedRefSpecs: []string{"refs/heads/main:refs/heads/main", "refs/heads/feature:refs/heads/feature"},
		},
		"mix of both refs and refspecs specified": {
			args:             Args{Parameters: []string{"a", "refs/heads/main:refs/heads/main", "refs/heads/feature:refs/heads/feature", "b"}, RootDir: gitDir},
			expectedRefSpecs: []string{"refs/heads/a:refs/heads/a", "refs/heads/main:refs/heads/main", "refs/heads/feature:refs/heads/feature", "refs/heads/b:refs/heads/b"},
		},
		"custom remote": {
			args:             Args{Parameters: []string{"git@example.com:/owner/repo", "main"}, RootDir: gitDir},
			expectedRefSpecs: []string{"refs/heads/main:refs/heads/main"},
		},
	}

	for name, test := range tests {
		refSpecs, err := DeterminePushedRefs(repo, test.args)
		if err != nil {
			t.Fatalf("unexpected error in test '%s': %s", name, err)
		}

		assert.Equal(t, test.expectedRefSpecs, refSpecs, fmt.Sprintf("unexpected result in test '%s'", name))
	}
}
