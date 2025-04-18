// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package luasandbox

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/gittuf/gittuf/internal/gitinterface"
	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	lua "github.com/yuin/gopher-lua"
)

var (
	testCtx = context.Background()
)

func TestNewLuaEnvironment(t *testing.T) {
	tmpDir := t.TempDir()
	repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

	environment, err := NewLuaEnvironment(testCtx, repo)
	defer environment.Cleanup()
	assert.Nil(t, err)
	assert.NotNil(t, environment)
}

func TestRunScript(t *testing.T) {
	t.Run("basic script", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

		environment, err := NewLuaEnvironment(testCtx, repo)
		defer environment.Cleanup()
		require.Nil(t, err)

		exitCode, err := environment.RunScript(string(artifacts.SampleHookScript), lua.LTable{})
		assert.Nil(t, err)
		assert.Equal(t, 0, exitCode)
	})
}

func TestAPIMatchRegex(t *testing.T) {
	tmpDir := t.TempDir()
	repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

	environment, err := NewLuaEnvironment(testCtx, repo)
	if err != nil {
		t.Fatal(err)
	}
	defer environment.Cleanup()

	t.Run("exact match", func(t *testing.T) {
		testScript := `
		local result = matchRegex("a", "a")
		return result
		`

		err = environment.lState.DoString(testScript)
		assert.Nil(t, err)

		result := environment.lState.Get(-1)
		environment.lState.Pop(1)

		assert.Equal(t, lua.LBool(true), result)
	})

	t.Run("no match", func(t *testing.T) {
		testScript := `
		local result = matchRegex("a", "b")
		return result
		`

		err = environment.lState.DoString(testScript)
		assert.Nil(t, err)

		result := environment.lState.Get(-1)
		environment.lState.Pop(1)

		assert.Equal(t, lua.LBool(false), result)
	})

	t.Run("partial match", func(t *testing.T) {
		testScript := `
		local result = matchRegex("ab", "aba")
		return result
		`

		err = environment.lState.DoString(testScript)
		assert.Nil(t, err)

		result := environment.lState.Get(-1)
		environment.lState.Pop(1)

		assert.Equal(t, lua.LBool(true), result)
	})

	t.Run("compilation failure", func(t *testing.T) {
		testScript := `
		local result = matchRegex("*(&^#%)", "aba")
		return result
		`

		err = environment.lState.DoString(testScript)
		assert.Nil(t, err)

		result := environment.lState.Get(-1)
		environment.lState.Pop(1)

		assert.Equal(t, lua.LString("Error: error parsing regexp: missing argument to repetition operator: `*`"), result)
	})
}

func TestAPIStrSplit(t *testing.T) {
	tmpDir := t.TempDir()
	repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)
	environment, err := NewLuaEnvironment(testCtx, repo)
	if err != nil {
		t.Fatal(err)
	}
	defer environment.Cleanup()

	t.Run("no separator", func(t *testing.T) {
		testScript := `
		local result = strSplit("hello\nworld")
		return result
		`

		err = environment.lState.DoString(testScript)
		assert.Nil(t, err)

		result := environment.lState.Get(-1)
		environment.lState.Pop(1)

		table := result.(*lua.LTable)

		assert.Equal(t, 2, table.Len())
		assert.Equal(t, lua.LString("hello"), table.Remove(1))
		assert.Equal(t, lua.LString("world"), table.Remove(1))
	})

	t.Run("with separator", func(t *testing.T) {
		testScript := `
		local result = strSplit("hello\\nworld", "\\n")
		return result
		`

		err = environment.lState.DoString(testScript)
		assert.Nil(t, err)

		result := environment.lState.Get(-1)
		environment.lState.Pop(1)

		table := result.(*lua.LTable)

		assert.Equal(t, 2, table.Len())
		assert.Equal(t, lua.LString("hello"), table.Remove(1))
		assert.Equal(t, lua.LString("world"), table.Remove(1))
	})
}

func TestAPIGitReadBlob(t *testing.T) {
	tmpDir := t.TempDir()
	repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

	environment, err := NewLuaEnvironment(testCtx, repo)
	if err != nil {
		t.Fatal(err)
	}

	defer environment.Cleanup()

	t.Run("text blob", func(t *testing.T) {
		contents := []byte("Hello, world!")
		blobID, err := repo.WriteBlob(contents)
		if err != nil {
			t.Fatal(err)
		}

		testScript := fmt.Sprintf(`
		local result = gitReadBlob("%s")
		return result
		`, blobID)

		err = environment.lState.DoString(testScript)
		assert.Nil(t, err)

		result := environment.lState.Get(-1)
		environment.lState.Pop(1)

		expectedValue := lua.LString(contents)

		assert.Equal(t, expectedValue, result)
	})

	t.Run("binary blob", func(t *testing.T) {
		contents := artifacts.GittufLogo
		blobID, err := repo.WriteBlob(contents)
		if err != nil {
			t.Fatal(err)
		}

		testScript := fmt.Sprintf(`
		local result = gitReadBlob("%s")
		return result
		`, blobID)

		err = environment.lState.DoString(testScript)
		assert.Nil(t, err)

		result := environment.lState.Get(-1)
		environment.lState.Pop(1)

		expectedValue := lua.LString(contents)

		assert.Equal(t, expectedValue, result)
	})
}

func TestAPIGitGetObjectSize(t *testing.T) {
	tmpDir := t.TempDir()
	repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

	contents := []byte("Hello, world!")
	blobID, err := repo.WriteBlob(contents)
	if err != nil {
		t.Fatal(err)
	}

	blobSize, err := repo.GetObjectSize(blobID)
	if err != nil {
		t.Fatal(err)
	}

	environment, err := NewLuaEnvironment(testCtx, repo)
	if err != nil {
		t.Fatal(err)
	}
	defer environment.Cleanup()

	testScript := fmt.Sprintf(`
	local result = gitGetObjectSize("%s")
	return result
	`, blobID)

	err = environment.lState.DoString(testScript)
	assert.Nil(t, err)

	result := environment.lState.Get(-1)
	environment.lState.Pop(1)

	expectedValue := lua.LString(strconv.FormatUint(blobSize, 10))

	assert.Equal(t, expectedValue, result)
}

func TestAPIGitGetTagTarget(t *testing.T) {
	tmpDir := t.TempDir()
	repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

	environment, err := NewLuaEnvironment(testCtx, repo)
	if err != nil {
		t.Fatal(err)
	}
	defer environment.Cleanup()

	treeBuilder := gitinterface.NewTreeBuilder(repo)

	// Write empty tree
	emptyTreeID, err := treeBuilder.WriteTreeFromEntries(nil)
	if err != nil {
		t.Fatal(err)
	}

	commitID, err := repo.Commit(emptyTreeID, "refs/heads/main", "Initial commit\n", true)
	if err != nil {
		t.Fatal(err)
	}

	tagID, err := repo.TagUsingSpecificKey(commitID, "test-tag", "test-tag\n", artifacts.SSHED25519Private)
	if err != nil {
		t.Fatal(err)
	}

	testScript := fmt.Sprintf(`
	local result = gitGetTagTarget("%s")
	return result
	`, tagID)

	err = environment.lState.DoString(testScript)
	assert.Nil(t, err)

	result := environment.lState.Get(-1)
	environment.lState.Pop(1)

	expectedValue := lua.LString(commitID.String())

	assert.Equal(t, expectedValue, result)
}

func TestAPIGitGetReference(t *testing.T) {
	tmpDir := t.TempDir()
	repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

	environment, err := NewLuaEnvironment(testCtx, repo)
	if err != nil {
		t.Fatal(err)
	}
	defer environment.Cleanup()

	treeBuilder := gitinterface.NewTreeBuilder(repo)

	// Write empty tree
	emptyTreeID, err := treeBuilder.WriteTreeFromEntries(nil)
	if err != nil {
		t.Fatal(err)
	}

	commitID, err := repo.Commit(emptyTreeID, "refs/heads/main", "Initial commit\n", true)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("main", func(t *testing.T) {
		ref := "main"

		testScript := fmt.Sprintf(`
		local result = gitGetReference("%s")
		return result
		`, ref)

		err = environment.lState.DoString(testScript)
		assert.Nil(t, err)

		result := environment.lState.Get(-1)
		environment.lState.Pop(1)

		expectedValue := lua.LString(commitID.String())

		assert.Equal(t, expectedValue, result)
	})

	t.Run("refs/heads/main", func(t *testing.T) {
		ref := "refs/heads/main"

		testScript := fmt.Sprintf(`
		local result = gitGetReference("%s")
		return result
		`, ref)

		err = environment.lState.DoString(testScript)
		assert.Nil(t, err)

		result := environment.lState.Get(-1)
		environment.lState.Pop(1)

		expectedValue := lua.LString(commitID.String())

		assert.Equal(t, expectedValue, result)
	})
}

func TestAPIGitGetAbsoluteReference(t *testing.T) {
	tmpDir := t.TempDir()
	repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

	environment, err := NewLuaEnvironment(testCtx, repo)
	if err != nil {
		t.Fatal(err)
	}
	defer environment.Cleanup()

	treeBuilder := gitinterface.NewTreeBuilder(repo)

	// Write empty tree
	emptyTreeID, err := treeBuilder.WriteTreeFromEntries(nil)
	if err != nil {
		t.Fatal(err)
	}

	_, err = repo.Commit(emptyTreeID, "refs/heads/main", "Initial commit\n", true)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("main", func(t *testing.T) {
		ref := "main"
		absRef := "refs/heads/main"

		testScript := fmt.Sprintf(`
		local result = gitGetAbsoluteReference("%s")
		return result
		`, ref)

		err = environment.lState.DoString(testScript)
		assert.Nil(t, err)

		result := environment.lState.Get(-1)
		environment.lState.Pop(1)

		expectedValue := lua.LString(absRef)

		assert.Equal(t, expectedValue, result)
	})

	t.Run("refs/heads/main", func(t *testing.T) {
		ref := "refs/heads/main"

		testScript := fmt.Sprintf(`
		local result = gitGetAbsoluteReference("%s")
		return result
		`, ref)

		err = environment.lState.DoString(testScript)
		assert.Nil(t, err)

		result := environment.lState.Get(-1)
		environment.lState.Pop(1)

		expectedValue := lua.LString(ref)

		assert.Equal(t, expectedValue, result)
	})
}

func TestAPIGitGetSymbolicReferenceTarget(t *testing.T) {
	tmpDir := t.TempDir()
	repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

	environment, err := NewLuaEnvironment(testCtx, repo)
	if err != nil {
		t.Fatal(err)
	}
	defer environment.Cleanup()

	treeBuilder := gitinterface.NewTreeBuilder(repo)

	// Write empty tree
	emptyTreeID, err := treeBuilder.WriteTreeFromEntries(nil)
	if err != nil {
		t.Fatal(err)
	}

	_, err = repo.Commit(emptyTreeID, "refs/heads/main", "Initial commit\n", true)
	if err != nil {
		t.Fatal(err)
	}

	ref := "HEAD"
	targetRef := "refs/heads/main"

	testScript := fmt.Sprintf(`
	local result = gitGetSymbolicReferenceTarget("%s")
	return result
	`, ref)

	err = environment.lState.DoString(testScript)
	assert.Nil(t, err)

	result := environment.lState.Get(-1)
	environment.lState.Pop(1)

	expectedValue := lua.LString(targetRef)

	assert.Equal(t, expectedValue, result)
}

func TestAPIGitGetCommitMessage(t *testing.T) {
	tmpDir := t.TempDir()
	repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

	environment, err := NewLuaEnvironment(testCtx, repo)
	if err != nil {
		t.Fatal(err)
	}
	defer environment.Cleanup()

	treeBuilder := gitinterface.NewTreeBuilder(repo)

	// Write empty tree
	emptyTreeID, err := treeBuilder.WriteTreeFromEntries(nil)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("basic message", func(t *testing.T) {
		message := "Initial commit"

		commitID, err := repo.Commit(emptyTreeID, "refs/heads/main", message, true)
		if err != nil {
			t.Fatal(err)
		}

		testScript := fmt.Sprintf(`
		local result = gitGetCommitMessage("%s")
		return result
		`, commitID)

		err = environment.lState.DoString(testScript)
		assert.Nil(t, err)

		result := environment.lState.Get(-1)
		environment.lState.Pop(1)

		expectedValue := lua.LString(message)

		assert.Equal(t, expectedValue, result)
	})

	t.Run("message with newline", func(t *testing.T) {
		message := `Initial
		commit`

		commitID, err := repo.Commit(emptyTreeID, "refs/heads/main", message, true)
		if err != nil {
			t.Fatal(err)
		}

		testScript := fmt.Sprintf(`
		local result = gitGetCommitMessage("%s")
		return result
		`, commitID)

		err = environment.lState.DoString(testScript)
		assert.Nil(t, err)

		result := environment.lState.Get(-1)
		environment.lState.Pop(1)

		expectedValue := lua.LString(message)

		assert.Equal(t, expectedValue, result)
	})
}

func TestAPIGitGetFilePathsChangedByCommit(t *testing.T) {
	tmpDir := t.TempDir()
	repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

	ref := "refs/heads/main"

	treeBuilder := gitinterface.NewTreeBuilder(repo)

	environment, err := NewLuaEnvironment(testCtx, repo)
	if err != nil {
		t.Fatal(err)
	}
	defer environment.Cleanup()

	blobIDs := []gitinterface.Hash{}
	for i := 0; i < 3; i++ {
		blobID, err := repo.WriteBlob([]byte(fmt.Sprintf("%d", i)))
		if err != nil {
			t.Fatal(err)
		}
		blobIDs = append(blobIDs, blobID)
	}

	t.Run("modify single file", func(t *testing.T) {
		treeA, err := treeBuilder.WriteTreeFromEntries([]gitinterface.TreeEntry{gitinterface.NewEntryBlob("a", blobIDs[0])})
		if err != nil {
			t.Fatal(err)
		}

		treeB, err := treeBuilder.WriteTreeFromEntries([]gitinterface.TreeEntry{gitinterface.NewEntryBlob("a", blobIDs[1])})
		if err != nil {
			t.Fatal(err)
		}

		_, err = repo.Commit(treeA, ref, "Test commit\n", false)
		if err != nil {
			t.Fatal(err)
		}

		cB, err := repo.Commit(treeB, ref, "Test commit\n", false)
		if err != nil {
			t.Fatal(err)
		}

		testScript := fmt.Sprintf(`
		local result = gitGetFilePathsChangedByCommit("%s")
		return result
		`, cB)

		err = environment.lState.DoString(testScript)
		assert.Nil(t, err)

		result := environment.lState.Get(-1)
		environment.lState.Pop(1)

		table := result.(*lua.LTable)

		assert.Equal(t, 1, table.Len())
		assert.Equal(t, lua.LString("a"), table.Remove(1))
	})

	t.Run("rename single file", func(t *testing.T) {
		treeA, err := treeBuilder.WriteTreeFromEntries([]gitinterface.TreeEntry{gitinterface.NewEntryBlob("a", blobIDs[0])})
		if err != nil {
			t.Fatal(err)
		}

		treeB, err := treeBuilder.WriteTreeFromEntries([]gitinterface.TreeEntry{gitinterface.NewEntryBlob("b", blobIDs[0])})
		if err != nil {
			t.Fatal(err)
		}

		_, err = repo.Commit(treeA, ref, "Test commit\n", false)
		if err != nil {
			t.Fatal(err)
		}

		cB, err := repo.Commit(treeB, ref, "Test commit\n", false)
		if err != nil {
			t.Fatal(err)
		}

		testScript := fmt.Sprintf(`
		local result = gitGetFilePathsChangedByCommit("%s")
		return result
		`, cB)

		err = environment.lState.DoString(testScript)
		assert.Nil(t, err)

		result := environment.lState.Get(-1)
		environment.lState.Pop(1)

		table := result.(*lua.LTable)

		assert.Equal(t, 2, table.Len())
		assert.Equal(t, lua.LString("a"), table.Remove(1))
		assert.Equal(t, lua.LString("b"), table.Remove(1))
	})
}

func TestAPIGitGetRemoteURL(t *testing.T) {
	tmpDir := t.TempDir()
	repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

	environment, err := NewLuaEnvironment(testCtx, repo)
	if err != nil {
		t.Fatal(err)
	}
	defer environment.Cleanup()

	remoteName := "origin"
	remoteURL := "git@example.com:repo.git"

	err = repo.AddRemote(remoteName, remoteURL)
	if err != nil {
		t.Fatal(err)
	}

	testScript := fmt.Sprintf(`
	local result = gitGetRemoteURL("%s")
	return result
	`, remoteName)

	err = environment.lState.DoString(testScript)
	assert.Nil(t, err)

	result := environment.lState.Get(-1)
	environment.lState.Pop(1)

	expectedValue := lua.LString(remoteURL)

	assert.Equal(t, expectedValue, result)
}
