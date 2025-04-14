// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

// This file contains modified code from the hub project, available at
// https://github.com/mislav/hub/blob/master/commands/args.go, and licensed
// under the MIT License

package args

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/gittuf/gittuf/internal/gitinterface"
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

var (
	ErrNoRemoteFound = errors.New("no remote found in Git configuration")
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

type GitConfig map[string]string

// ProcessArgs takes in the arguments from the commandline, and then nicely
// organizes them into an Args struct. This should only be called with 1 or more
// arguments, excluding the base one (i.e. the executable name)
func ProcessArgs(args []string) Args {
	if len(args) < 1 {
		return Args{
			GlobalFlags: nil,
			Command:     "",
			Parameters:  nil,
			GitDir:      defaultGitDir,
			RootDir:     defaultRootDir,
		}
	}

	// Find the command's index
	cmdIndex, configIndex, chdirIndex, girDirIndex := locateCommand(args)

	gitDir := defaultGitDir
	rootDir := defaultRootDir
	if chdirIndex > 0 {
		gitDir = args[chdirIndex] + "/" + defaultGitDir
		rootDir = args[chdirIndex]
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
		RootDir:     rootDir,
	}
}

// DetermineRemote attempts to find out what the remote is that the user would
// like to push to. If there is no information to this regard in the original
// command invocation, "origin" and the accompanying URL are assumed.
func DetermineRemote(args Args) (string, string, error) {
	repo, err := gitinterface.LoadRepository(args.RootDir)
	if err != nil {
		return "", "", err
	}

	config, err := repo.GetGitConfig()
	if err != nil {
		return "", "", err
	}

	// If there are no args, get the default remote from the git config
	if len(args.Parameters) == 0 {
		remoteName, remoteURL, err := getDefaultRemoteFromConfig(config)
		if err != nil {
			return "", "", err
		}
		return remoteName, remoteURL, nil
	}

	// Iterate args to find the remote
	for _, arg := range args.Parameters {
		if strings.HasPrefix(arg, flagPrefix) {
			continue
		}

		if isValidRemoteName(arg, config) {
			remoteURL := config["remote."+arg+".url"]
			return arg, remoteURL, nil
		}

		// The remote must be specified before refspecs so this should work
		if strings.Contains(arg, "https://") || strings.Contains(arg, "http://") || (strings.Contains(arg, "@") && strings.Contains(arg, ":")) {
			return arg, arg, nil
		}
	}

	// If no remote was found, get the default remote from the git config
	remoteName, remoteURL, err := getDefaultRemoteFromConfig(config)
	if err != nil {
		return "", "", err
	}

	return remoteName, remoteURL, nil
}

// DeterminePushedRefs parses the git references from the given command-line
// arguments.
func DeterminePushedRefs(repo *gitinterface.Repository, gitArgs Args) ([]string, error) {
	// There are a few possible cases for how the invocation of push can look
	// like. The following examples are taken from the Git manpage:
	// git push
	// git push origin
	// git push origin :
	// git push origin master
	// git push origin HEAD
	// git push mothership master:satellite/master dev:satellite/dev
	// git push origin HEAD:master
	// git push origin master:refs/heads/experimental
	// git push origin :experimental
	// git push origin +dev:master

	// There are also cases where a custom remote may be used:
	// git push git@example.com:owner/repo
	// git push git@example.com:owner/repo HEAD
	// git push git@example.com:owner/repo refs/heads/main:refs/heads/main

	// The good news here is that the remote must come as the first parameter,
	// if it is supplied at all.

	// As we can see, there are a ton of combinations to parse through. This
	// function must accommodate all of these cases, and so follows the
	// following algorithm:

	// 0. For all arguments, if a "--" is present as the first part of the arg
	// string, then we skip over it as it's a flag.
	// 1. If len(parameters) == 0, then we resolve HEAD and create a refspec
	// with it on both the local and remote part.
	// 2. Iterate through all the args supplied on the commandline. For each
	// arg:
	//    a) Check if the arg contains a ":". If it does and it is the first
	//   	 arg, then check if "refs/" is present anywhere in the arg. If it
	//       is, then it's likely a refspec. Add it to the list of refspecs.
	//    b) If the arg does not contain a ":" and it is the first arg, see if
	//       we can resolve a remote in the Git configuration for this arg. If
	//       we can, then skip over this arg. If not, then determine the refspec
	//       for this arg.
	//    c) If the arg does not contain a ":" and it is not the first arg, then
	//       attempt to determine the refspec for it.

	if len(gitArgs.Parameters) == 0 {
		headTarget, err := repo.GetSymbolicReferenceTarget("HEAD")
		if err != nil {
			return nil, err
		}
		refSpec := fmt.Sprintf("%s:%s", headTarget, headTarget)
		return []string{refSpec}, nil
	}

	var refNames []string

	for i, arg := range gitArgs.Parameters {
		if i == 0 {
			possibleRemote, _, _ := DetermineRemote(gitArgs)
			if possibleRemote == arg {
				// If this is the only argument, then we must assume HEAD
				if len(gitArgs.Parameters) == 1 {
					headTarget, err := repo.GetSymbolicReferenceTarget("HEAD")
					if err != nil {
						return nil, err
					}
					refSpec := fmt.Sprintf("%s:%s", headTarget, headTarget)
					return []string{refSpec}, nil
				}
				continue
			}
		}

		if strings.HasPrefix(arg, flagPrefix) {
			continue
		}

		if strings.Contains(arg, ":") {
			refNames = append(refNames, arg)
		} else {
			refSpec, err := repo.RefSpec(arg, "", true)
			if err != nil {
				return nil, err
			}
			refNames = append(refNames, refSpec)
		}
	}

	return refNames, nil
}

// isValidRemoteName checks if the given name is a valid remote name in the
// repository's Git configuration.
func isValidRemoteName(name string, config GitConfig) bool {
	return config["remote."+name+".url"] != ""
}

// getDefaultRemoteFromConfig returns the default remote from the given config
// map. If no remote is found, "origin" is returned.
func getDefaultRemoteFromConfig(config GitConfig) (string, string, error) {
	for key := range config {
		if strings.HasPrefix(key, "remote.") && strings.HasSuffix(key, ".url") {
			return strings.TrimSuffix(strings.TrimPrefix(key, "remote."), ".url"), config[key], nil
		}
	}

	// If there isn't a remote, return an error
	return "", "", ErrNoRemoteFound
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
			slog.Debug(fmt.Sprintf("Decoded command as '%s'", args[idx]))
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
