// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"fmt"
	"os/exec"
	"path"
	"strings"
)

// ResetDueToError reverses a change applied to a ref to the specified target
// ID. It is used to ensure a gittuf operation is atomic: if a gittuf operation
// fails, any changes made to the repository in refs/gittuf can be rolled back.
// Worktrees are not updated.
func (r *Repository) ResetDueToError(cause error, refName string, commitID Hash) error {
	if err := r.SetReference(refName, commitID); err != nil {
		return fmt.Errorf("unable to reset %s to %s, caused by following error: %w", refName, commitID.String(), cause)
	}
	return cause
}

func RemoteRef(refName, remoteName string) string {
	var remotePath string
	switch {
	case strings.HasPrefix(refName, BranchRefPrefix):
		// refs/heads/<path> -> refs/remotes/<remote>/<path>
		rest := strings.TrimPrefix(refName, BranchRefPrefix)
		remotePath = path.Join(RemoteRefPrefix, remoteName, rest)
	case strings.HasPrefix(refName, TagRefPrefix):
		// refs/tags/<path> -> refs/tags/<path>
		remotePath = refName
	default:
		// refs/<path> -> refs/remotes/<remote>/<path>
		rest := strings.TrimPrefix(refName, RefPrefix)
		remotePath = path.Join(RemoteRefPrefix, remoteName, rest)
	}

	return remotePath
}

// IsNiceGitVersion determines whether the version of git is "nice". Certain Git
// subcommands that gittuf uses were added in newer versions than some common
// client versions. Instead of using a workaround for all clients, we determine
// if we can use the newer features or instead need to use workarounds.
func isNiceGitVersion() (bool, error) {
	cmd := exec.Command("git", "--version")
	output, err := cmd.Output()
	if err != nil {
		return false, err
	}

	versionString := strings.TrimPrefix(strings.TrimSpace(string(output)), "git version ")

	var major, minor, patch int
	_, err = fmt.Sscanf(versionString, "%d.%d.%d", &major, &minor, &patch)
	if err != nil {
		return false, err
	}

	if major >= 2 && minor >= 38 {
		return true, nil
	}
	return false, nil
}
