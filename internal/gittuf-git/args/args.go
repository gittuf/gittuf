// SPDX-License-Identifier: Apache-2.0

// This file contains modified code from the hub project, available at
// https://github.com/mislav/hub/blob/master/commands/args.go, and licensed
// under the MIT License

package args

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	noopFlag       = "--noop"
	versionFlag    = "--version"
	listCmds       = "--list-cmds="
	helpFlag       = "--help"
	configFlag     = "-c"
	chdirFlag      = "-C"
	gitDirFlag     = "--git-dir"
	flagPrefix     = "-"
	defaultRemote  = "origin"
	defaultGitDir  = ".git"
	defaultRootDir = "."
)

// Args stores any global flags for the invocation of Git (i.e. those before the
// operation), the command/operation, and the parameters to said
// command/operation.
type Args struct {
	GlobalFlags []string
	Command     string
	Parameters  []string
	ConfigIdx   int
	ChdirIdx    int
	GitDir      string
	RootDir     string
}

// ProcessArgs takes in the arguments from the commandline, and then nicely
// organizes them into an Args struct. This should only be called with 1 or more
// arguments, excluding the base one (i.e. the executable name)
func ProcessArgs(args []string) Args {
	if len(args) < 1 {
		return Args{
			GlobalFlags: nil,
			Command:     "",
			Parameters:  nil,
		}
	}

	// Find the command's index
	cmdIndex, configIndex, chdirIndex, girDirIndex := locateCommand(args)

	gitDir := defaultGitDir
	RootDir := defaultRootDir
	if chdirIndex > 0 {
		gitDir = args[chdirIndex] + "/" + defaultGitDir
		RootDir = args[chdirIndex]
	}

	if girDirIndex > 0 {
		gitDir = args[girDirIndex]
	}

	return Args{
		GlobalFlags: args[0:cmdIndex],
		Command:     args[cmdIndex],
		Parameters:  args[cmdIndex+1:],
		ConfigIdx:   configIndex,
		ChdirIdx:    chdirIndex,
		GitDir:      gitDir,
		RootDir:     RootDir,
	}
}

// DetermineRemote attempts to find out what the remote is that the user would
// like to push to. If there is no information to this regard in the original
// command invocation, "origin" is assumed.
func DetermineRemote(args []string, gitDir string) (string, error) {
	// If there are no args, get the default remote from the git config
	if len(args) == 0 {
		config, err := GetGitConfig(gitDir)
		if err != nil {
			return defaultRemote, err
		}

		remote := getDefaultRemoteFromConfig(config)
		return remote, nil
	}

	// Iterate args to find the remote
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			continue
		}

		if strings.Contains(arg, ":") {
			parts := strings.Split(arg, ":")
			if len(parts) > 0 {
				return parts[0], nil
			}
		}

		if isValidRemoteName(arg, gitDir) {
			return arg, nil
		}
	}

	// If no remote was found, get the default remote from the git config
	config, err := GetGitConfig(gitDir)
	if err != nil {
		return defaultRemote, err
	}

	remote := getDefaultRemoteFromConfig(config)

	return remote, nil
}

// isValidRemoteName checks if the given name is a valid remote name in the
// repository located at gitDir.
func isValidRemoteName(name string, gitDir string) bool {
	// Check if the remote directory exists in refs/remotes
	remoteDir := filepath.Join(gitDir, "refs", "remotes", name)
	_, err := os.Stat(remoteDir)
	return err == nil
}

// GetGitConfig reads the applicable Git config for gitDir and returns it as a
// map. The keys are the config names in lowercase
func GetGitConfig(gitDir string) (map[string]string, error) {
	cmd := exec.Command("git", "-C", gitDir, "config", "--local", "--get-regexp", ".*")
	stdOut, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}

	stdOut = []byte(strings.TrimSpace(string(stdOut)))

	config := map[string]string{}

	lines := strings.Split(strings.TrimSpace(string(stdOut)), "\n")
	for _, line := range lines {
		split := strings.Split(line, " ")
		if len(split) < 2 {
			continue
		}
		config[strings.ToLower(split[0])] = strings.Join(split[1:], " ")
	}

	return config, nil
}

// getDefaultRemoteFromConfig returns the default remote from the given config
// map. If no remote is found, "origin" is returned.
func getDefaultRemoteFromConfig(config map[string]string) string {
	for key := range config {
		if strings.HasPrefix(key, "remote.") && strings.HasSuffix(key, ".url") {
			return strings.TrimSuffix(strings.TrimPrefix(key, "remote."), ".url")
		}
	}

	return defaultRemote
}

// locateCommand attempts to find where the main Git operation is in the
// arguments. We need to do this as flags can be specified before the command.
// In addition, it returns the location of any specified config directory and
// change directory operation.
// (The output format is (commandIndex, configIndex, chdirIndex))
func locateCommand(args []string) (int, int, int, int) {
	skip := false
	idx, config, chdir, gitDirInd := 0, 0, 0, 0

	for i, arg := range args {
		switch {
		case skip:
			// If skip is set, then the current arg is a parameter to the
			// previous arg. As we've processed it below, increment idx and
			// proceed to the next arg.
			idx = i + 1
			skip = false
		case arg == versionFlag || arg == helpFlag || arg == gitDirFlag ||
			strings.HasPrefix(arg, listCmds) || !strings.HasPrefix(arg, flagPrefix):
			// If the arg is the version flag, a help / list cmd flag, or does
			// not have the "-" prefix, we've reached the location of the
			// operation.
			return idx, config, chdir, gitDirInd
		default:
			// Otherwise, we need to keep processing args. Advance the index and
			// check if the arg is one of interest.
			idx = i + 1
			switch arg {
			case configFlag:
				// The current arg indicates that the next value will be a
				// config file. Set the config index appropriately.
				config = idx
				skip = true
			case chdirFlag:
				// The current arg indicates that the next value will be a
				// directory. Set the config index appropriately.
				chdir = idx
				skip = true
			case gitDirFlag:
				// The current arg indicates that the next value will be a
				// directory. Set the config index appropriately.
				gitDirInd = idx
				skip = true
			}
		}
	}
	return idx, config, chdir, gitDirInd
}
