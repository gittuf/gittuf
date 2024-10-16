// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

// This file contains modified code from the lua-sandbox project, available at
// https://github.com/kikito/lua-sandbox/blob/master/sandbox.lua, and licensed
// under the MIT License

package lua

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gittuf/gittuf/internal/common/set"
	lua "github.com/yuin/gopher-lua"
)

var setupEnvironment = `
repositoryInformation = {}
repositoryInformation["user.name"] = "Jane Doe"
repositoryInformation["user.email"] = "jane.doe@example.com"
`

// Lua sandbox helper functions and wrappers functions of provided Go APIs
var helpers = `
function splitString(str, sep)
	if sep == nil then
		sep = "\n"
	end

	local lines = {}
	for line in string.gmatch(str, "([^"..sep.."]+)") do
		table.insert(lines, line)
	end

	return lines
end

function goLinter(path) return runFunc("goLinter", path, nil) end

function removeFile(path) return deleteFile(path) end

function scanDir(path, recursive) return goScanDir(path, recursive) end

function getRootDir() return allowedDir end

function execute(command) 
	output = goExecute(command)
	print(output)
	return output
end

`

var allowedExecutables = set.NewSetFromItems[string]("golangci-lint", "git")
var allowedDir = getGitRoot()

// NewLuaEnvironment creates a new Lua sandbox with the specified environment
func NewLuaEnvironment(luaMode string) (*lua.LState, error) {
	// Create a new Lua state
	lState := lua.NewState(lua.Options{SkipOpenLibs: true})

	// Load default safe libraries
	modules := []struct {
		n string
		f lua.LGFunction
	}{
		{lua.LoadLibName, lua.OpenPackage}, // Must be first
		{lua.BaseLibName, lua.OpenBase},
		{lua.TabLibName, lua.OpenTable},
		{lua.StringLibName, lua.OpenString},
		{lua.MathLibName, lua.OpenMath},
		{lua.CoroutineLibName, lua.OpenCoroutine},
	}

	switch luaMode {
	case "lessStrict":
		modules = append(modules, struct {
			n string
			f lua.LGFunction
		}{lua.OsLibName, lua.OpenOs})
		setTimeOut(context.Background(), lState, 5000)
	case "default":
		// Todo: Specify a default mode for lua sandbox
		enableOnlySafeFunctions(lState)
		setInstructionQuota(lState, int64(100000))
	}

	// Load the modules to the Lua state
	for _, pair := range modules {
		if err := lState.CallByParam(lua.P{
			Fn:      lState.NewFunction(pair.f),
			NRet:    0,
			Protect: true,
		}, lua.LString(pair.n)); err != nil {
			panic(err)
		}
	}

	// Load configuration into the Lua state
	if err := lState.DoString(setupEnvironment); err != nil {
		return nil, err
	}

	// Register the Go functions with the Lua state
	lState.SetGlobal("deleteFile", lState.NewFunction(deleteFile))
	lState.SetGlobal("goExecute", lState.NewFunction(goExecute))
	lState.SetGlobal("goScanDir", lState.NewFunction(goScanDir))
	lState.SetGlobal("allowedDir", lua.LString(allowedDir))

	// Load the pre-written pure Lua helper functions into the Lua state
	if err := lState.DoString(helpers); err != nil {
		return nil, err
	}

	return lState, nil
}

// Disable all functions in the specified list that are not safe
func enableOnlySafeFunctions(l *lua.LState) {
	//-- List of unsafe packages/functions:
	// -- * string.rep: can be used to allocate millions of bytes in 1 operation
	// -- * {set|get}metatable: can be used to modify the metatable of global objects (strings, integers)
	// -- * collectgarbage: can affect performance of other systems
	// -- * dofile: can access the server filesystem
	// -- * _G: It has access to everything. It can be mocked to other things though.
	// -- * load{file|string}: All unsafe because they can grant acces to global env
	// -- * raw{get|set|equal}: Potentially unsafe
	// -- * module|require|module: Can modify the host settings
	// -- * string.dump: Can display confidential server info (implementation of functions)
	// -- * math.randomseed: Can affect the host system
	// -- * io.*, os.*: Most stuff there is unsafe, see below for exceptions

	// Disable all unsafe functions
	l.SetGlobal("dofile", lua.LNil)
	l.SetGlobal("load", lua.LNil)
	l.SetGlobal("loadfile", lua.LNil)
	l.SetGlobal("loadstring", lua.LNil)
	l.SetGlobal("require", lua.LNil)
	l.SetGlobal("module", lua.LNil)
	l.SetGlobal("collectgarbage", lua.LNil)
	l.SetGlobal("rawget", lua.LNil)
	l.SetGlobal("rawset", lua.LNil)
	l.SetGlobal("rawequal", lua.LNil)
	l.SetGlobal("setmetatable", lua.LNil)
	l.SetGlobal("getmetatable", lua.LNil)
	l.SetGlobal("_G", lua.LNil)
	l.SetGlobal("os", lua.LNil)
	l.SetGlobal("io", lua.LNil)
	l.SetGlobal("debug", lua.LNil)
	l.SetGlobal("package", lua.LNil)

	if strMod, ok := l.GetGlobal(lua.StringLibName).(*lua.LTable); ok {
		strMod.RawSetString("rep", lua.LNil)
		strMod.RawSetString("dump", lua.LNil)
		protectModule(l, strMod, lua.StringLibName)
	}

	// Load protected modules with only safe functions
	if mathMod, ok := l.GetGlobal(lua.MathLibName).(*lua.LTable); ok {
		mathMod.RawSetString("randomseed", lua.LNil)
		protectModule(l, mathMod, lua.MathLibName)
	}

	if coroMod, ok := l.GetGlobal(lua.CoroutineLibName).(*lua.LTable); ok {
		protectModule(l, coroMod, lua.CoroutineLibName)
	}

	if tabMod, ok := l.GetGlobal(lua.TabLibName).(*lua.LTable); ok {
		protectModule(l, tabMod, lua.TabLibName)
	}

	if baseMod, ok := l.GetGlobal(lua.BaseLibName).(*lua.LTable); ok {
		protectModule(l, baseMod, lua.BaseLibName)
	}
}

// Protect the specified module from being modified by setting a protected
// metatable with __newindex and __metatable fields
func protectModule(l *lua.LState, tbl *lua.LTable, moduleName string) {
	mt := l.NewTable()
	l.SetMetatable(tbl, mt)
	l.SetField(mt, "__newindex", l.NewFunction(func(l *lua.LState) int {
		varName := l.ToString(2)
		l.RaiseError("attempt to modify read-only table '%s.%s'", moduleName, varName)
		return 0
	}))
	l.SetField(mt, "__metatable", lua.LString("protected"))
}

// Set the instruction quota for the Lua state
func setInstructionQuota(l *lua.LState, quota int64) {
	// TODO: Write pure lua function and call it from here
	if err := l.DoString(fmt.Sprintf("setInstructionQuota(%d)", quota)); err != nil {
		panic(err)
	}
}

// Set the timeout for the Lua state
func setTimeOut(ctx context.Context, lState *lua.LState, timeOut int) {
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeOut)*time.Second)
	defer cancel()

	lState.SetContext(ctx)
}

// runCommand parse the function and run only supported functions
func goExecute(l *lua.LState) int {
	command := l.ToString(1)
	commands := strings.Split(command, " ")

	// TODO: Verify the integrity of the executable

	var executable string
	var args []string

	// TODO: Make the args parsing more robust
	executable = commands[0]
	args = commands[1:]

	if !allowedExecutables.Has(executable) {
		l.Push(lua.LString("Error: Executable not allowed"))
		return 1
	}

	// execute command and capture output
	cmd := exec.Command(executable, args...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		l.Push(lua.LString(err.Error()))
		return 1
	}

	// Return output to Lua
	l.Push(lua.LString(string(output)))
	return 1
}

// getGitRoot returns the root directory of the git repository
func getGitRoot() string {
	// Check if the current directory is a git repository
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	output, err := cmd.CombinedOutput()
	if err != nil || strings.TrimSpace(string(output)) != "true" {
		// Get the path to the .git directory
		cmd = exec.Command("git", "rev-parse", "--git-dir")
		output, err = cmd.CombinedOutput()
		if err != nil {
			return ""
		}
		gitDir := strings.TrimSpace(string(output))
		if gitDir == "." || gitDir == ".git" {
			// If the .git directory is the current directory
			// then the root directory is the parent directory
			cmd = exec.Command("git", "rev-parse", "--show-cdup")
			output, err = cmd.CombinedOutput()
			if err != nil {
				return ""
			}
			relativeRootDir := strings.TrimSpace(string(output))
			if relativeRootDir == "" {
				relativeRootDir = "."
			}
			absoluteRootDir, err := filepath.Abs(relativeRootDir)
			if err != nil {
				return ""
			}
			return absoluteRootDir
		}
		absoluteGitDir, err := filepath.Abs(gitDir)
		if err != nil {
			return ""
		}
		return absoluteGitDir
	}

	// Get the root directory of the git repository if the current directory
	// is already inside the working tree
	cmd = exec.Command("git", "rev-parse", "--show-toplevel")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return ""
	}
	rootDir := strings.TrimSpace(string(output))
	absoluteRootDir, err := filepath.Abs(rootDir)
	if err != nil {
		return ""
	}
	return absoluteRootDir
}

// Check if the path is allowed to access
func isPathAllowed(path string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	absAllowedDir, _ := filepath.Abs(allowedDir)
	return strings.HasPrefix(absPath, absAllowedDir)
}

// A wrapper for lua environment to run protected os.Remove function
func deleteFile(l *lua.LState) int {
	filePath := l.ToString(1)

	if !isPathAllowed(filePath) {
		l.Push(lua.LString("Error: Access to this file is not allowed"))
		return 1
	}

	err := os.Remove(filePath)
	if err != nil {
		l.Push(lua.LString(fmt.Sprintf("Error deleting file: %s", err.Error())))
		return 1
	}

	l.Push(lua.LString("File deleted successfully"))
	return 1
}

// Return all filenames in the specified directory, take a second argument to
// specify if the scan should be recursive
func goScanDir(l *lua.LState) int {
	dirPath := l.ToString(1)
	recursive := l.ToBool(2)
	if !isPathAllowed(dirPath) {
		l.Push(lua.LString("Error: Access to this directory is not allowed"))
		return 1
	}

	var files []string
	// Recursively scan the directory
	if recursive {
		err := filepath.Walk(dirPath, func(path string, _ os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			files = append(files, path)
			return nil
		})
		if err != nil {
			l.Push(lua.LString(fmt.Sprintf("Error scanning directory: %s", err.Error())))
			return 1
		}
	} else {
		// Return all files in the current directory without entering subdirectories
		fileInfo, err := os.ReadDir(dirPath)
		if err != nil {
			l.Push(lua.LString(fmt.Sprintf("Error scanning directory: %s", err.Error())))
			return 1
		}
		for _, file := range fileInfo {
			files = append(files, file.Name())
		}
	}

	// Return all scanned files as a string to the Lua sandbox
	l.Push(lua.LString(strings.Join(files, "\n")))
	return 1
}
