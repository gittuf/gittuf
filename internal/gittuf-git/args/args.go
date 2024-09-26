// SPDX-License-Identifier: Apache-2.0

// This file contains modified code from the hub project, available at
// https://github.com/mislav/hub/blob/master/commands/args.go, and licensed
// under the MIT License

package args

import (
	"strings"
)

const (
	noopFlag      = "--noop"
	versionFlag   = "--version"
	listCmds      = "--list-cmds="
	helpFlag      = "--help"
	configFlag    = "-c"
	chdirFlag     = "-C"
	flagPrefix    = "-"
	defaultRemote = "origin"
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
	cmdIndex, configIndex, chdirIndex := locateCommand(args)

	// Organize and return args
	return Args{
		GlobalFlags: args[0:cmdIndex],
		Command:     args[cmdIndex],
		Parameters:  args[cmdIndex+1:],
		ConfigIdx:   configIndex,
		ChdirIdx:    chdirIndex,
	}
}

// DetermineRemote attempts to find out what the remote is that the user would
// like to push to. If there is no information to this regard in the original
// command invocation, "origin" is assumed.
func DetermineRemote(args []string) string {
	// TODO: This should be more robust and thourougly tested...
	if len(args) > 0 && args[0] != "origin" {
		if len(args) == 1 {
			return args[0]
		}
		for i, arg := range args {
			if arg == ":" {
				return args[i+1]
			}
		}
	}
	return defaultRemote
}

// locateCommand attempts to find where the main Git operation is in the
// arguments. We need to do this as flags can be specified before the command.
// In addition, it returns the location of any specified config directory and
// change directory operation.
// (The output format is (commandIndex, configIndex, chdirIndex))
func locateCommand(args []string) (int, int, int) {
	skip := false
	idx, config, chdir := 0, 0, 0

	for i, arg := range args {
		switch {
		case skip:
			// If skip is set, then the current arg is a parameter to the
			// previous arg. As we've processed it below, increment idx and
			// proceed to the next arg.
			idx = i + 1
			skip = false
		case arg == versionFlag || arg == helpFlag || strings.HasPrefix(arg, listCmds) || !strings.HasPrefix(arg, flagPrefix):
			// If the arg is the verion flag, a help / list cmd flag, or does
			// not have the "-" prefix, we've reached the location of the
			// operation.
			return idx, config, chdir
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
			}
		}
	}
	return idx, config, chdir
}
