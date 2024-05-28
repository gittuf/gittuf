package common

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// Copy all files from source path to destination path and set permissions
func copyFiles(src string, dst string, perm fs.FileMode) error {
	files, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", src, err)
	}
	for _, file := range files {
		data, err := os.ReadFile(filepath.Join(src, file.Name()))
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", file.Name(), err)
		}
		dstPath := filepath.Join(dst, file.Name())
		err = os.WriteFile(dstPath, data, perm)
		if err != nil {
			return fmt.Errorf("failed to write %s: %w", dstPath, err)
		}
	}
	return nil
}

// Create and return path to temporary test directory, including copies of
// ssh test keys, with restrictive permissions, as required by ssh-keygen.
func TestSSHKeys(tb testing.TB) string {
	keys := "../../testartifacts/testdata/keys/ssh"
	testDir := tb.TempDir()

	if err := copyFiles(keys, testDir, 0600); err != nil {
		tb.Fatal(err)
	}

	return testDir
}

var TestScripts = "../../testartifacts/testdata/scripts"
