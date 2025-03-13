// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

// This file contains modified code from the lua-sandbox project, available at
// https://github.com/kikito/lua-sandbox/blob/master/lua, and licensed
// under the MIT License

package luasandbox

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	lua "github.com/yuin/gopher-lua"
)

// This file (api.go) contains functions that are exposed to lua programs.

// Lua sandbox helper functions in pure Lua
var pureLuaHelpers = `
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
`

// Lua helpers map, mapping Lua function names to Go functions
var luaHelpersMap = map[string]lua.LGFunction{
	"regexMatch":            goRegexMatch,
	"readFile":              goReadFile,
	"getDiff":               goGetDiff,
	"getWorkTree":           goGetWorkTree,
	"checkAddedLargeFile":   goCheckAddedLargeFile,
	"checkMergeConflict":    goCheckMergeConflict,
	"checkJSON":             goCheckJSON,
	"checkNoCommitOnBranch": goCheckNoCommitOnBranch,
	"getGitObject":          goGetGitObject,
	"getGitObjectSize":      goGetGitObjectSize,
	"getGitObjectHash":      goGetGitObjectHash,
	"getGitObjectPath":      goGetGitObjectPath,
	"regexMatchGitDiff":     goRegexMatchGitDiff,
}

func registerAPIFunctions(lState *lua.LState) (*lua.LState, error) {
	// Set global variables for the Lua state
	lState.SetGlobal("hookParameters", lua.LString(""))
	lState.SetGlobal("hookExitCode", lua.LNumber(0))
	// lState.SetGlobal("allowedExecutables", lua.LString(strings.Join(allowedModules, ",")))

	// Register the pure Lua helper functions
	if err := lState.DoString(pureLuaHelpers); err != nil {
		return nil, err
	}

	// Register the Go functions
	for name, fn := range luaHelpersMap {
		lState.SetGlobal(name, lState.NewFunction(fn))
	}

	return lState, nil
}

// goReadFile reads the content of a file and returns it as a string
func goReadFile(l *lua.LState) int {
	filePath := l.ToString(1)
	if !isPathAllowed(filePath) {
		l.Push(lua.LString("Error: Access to this file is not allowed"))
		return 1
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		l.Push(lua.LString(fmt.Sprintf("Error reading file: %s", err.Error())))
		return 1
	}

	l.Push(lua.LString(string(data)))
	return 1
}

// goGetDiff retrieves the git diff output
func goGetDiff(l *lua.LState) int {
	output, err := getGitDiffOutput()
	if err != nil {
		l.Push(lua.LString(err.Error()))
	}
	l.Push(lua.LString(output))

	return 1
}

// goGetWorkTree retrieves the list of files in the git work tree
func goGetWorkTree(l *lua.LState) int {
	output, err := getWorkTreeFiles()

	if err != nil {
		l.Push(lua.LString(err.Error()))
		return 1
	}

	l.Push(lua.LString(strings.Join(output, "\n")))
	return 1
}

// goCheckAddedLargeFile checks for large files in the git diff output
func goCheckAddedLargeFile(l *lua.LState) int {
	maxSizeKB := l.ToInt(1)
	enforceAll := l.ToBool(2)

	var files []string
	var err error

	if enforceAll {
		files, err = getWorkTreeFiles()
	} else {
		files, err = getGitDiffFiles()
	}

	if err != nil {
		l.Push(lua.LString(fmt.Sprintf("Error: %s", err.Error())))
		return 1
	}

	largeFiles := l.NewTable()
	for _, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			continue
		}
		if info.Size() > int64(maxSizeKB*1024) {
			largeFiles.Append(lua.LString(file))
		}
	}

	l.Push(largeFiles)
	return 0
}

// goCheckMergeConflict checks for merge conflicts in the git diff output
func goCheckMergeConflict(l *lua.LState) int {
	var files []string
	var err error

	files, err = getGitDiffFiles()

	if err != nil {
		l.Push(lua.LString(fmt.Sprintf("Error: %s", err.Error())))
		return 1
	}

	conflictFiles := l.NewTable()
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		if bytes.Contains(data, []byte("<<<<<<< ")) ||
			bytes.Contains(data, []byte("======= ")) ||
			bytes.Contains(data, []byte(">>>>>>> ")) {
			conflictFiles.Append(lua.LString(file))
		}
	}

	l.Push(conflictFiles)
	return 0
}

// goCheckJSON checks for valid JSON files in the git diff output
func goCheckJSON(l *lua.LState) int {
	var files []string
	var err error

	files, err = getWorkTreeFiles()

	if err != nil {
		l.Push(lua.LString(fmt.Sprintf("Error: %s", err.Error())))
		return 1
	}

	jsonFiles := l.NewTable()
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		if json.Valid(data) {
			jsonFiles.Append(lua.LString(file))
		}
	}

	l.Push(jsonFiles)
	return 0
}

// goCheckNoCommitOnBranch checks if the current branch is protected
func goCheckNoCommitOnBranch(l *lua.LState) int {
	protectedBranches := l.ToTable(1)
	patterns := l.ToTable(2)

	branchName, err := getCurrentBranchName()
	if err != nil {
		l.Push(lua.LString(fmt.Sprintf("Error: %s", err.Error())))
		return 1
	}

	for i := 1; i <= protectedBranches.Len(); i++ {
		if branchName == protectedBranches.RawGetInt(i).String() {
			l.Push(lua.LBool(true))
			return 1
		}
	}

	for i := 1; i <= patterns.Len(); i++ {
		pattern := patterns.RawGetInt(i).String()
		matched, err := regexp.MatchString(pattern, branchName)
		if err != nil {
			l.Push(lua.LString(fmt.Sprintf("Error: %s", err.Error())))
			return 1
		}
		if matched {
			l.Push(lua.LBool(true))
			return 1
		}
	}

	l.Push(lua.LBool(false))
	return 0
}

// goGetGitObject retrieves the content of a git object
func goGetGitObject(l *lua.LState) int {
	object := l.ToString(1)

	cmd := exec.Command("git", "cat-file", "-p", object)
	output, err := cmd.CombinedOutput()
	if err != nil {
		l.Push(lua.LString(err.Error()))
		return 1
	}

	l.Push(lua.LString(string(output)))
	return 0
}

// goGetGitObject retrieves the content of a git object
func goGetGitObjectSize(l *lua.LState) int {
	object := l.ToString(1)

	cmd := exec.Command("git", "cat-file", "-s", object)
	output, err := cmd.CombinedOutput()
	if err != nil {
		l.Push(lua.LString(err.Error()))
		return 1
	}

	l.Push(lua.LString(string(output)))
	return 0
}

// goGetGitObjectHash retrieves the hash of a git object
func goGetGitObjectHash(l *lua.LState) int {
	object := l.ToString(1)

	cmd := exec.Command("git", "hash-object", "-w", object)
	output, err := cmd.CombinedOutput()
	if err != nil {
		l.Push(lua.LString(err.Error()))
		return 1
	}

	l.Push(lua.LString(string(output)))
	return 0
}

// goGetGitObjectPath retrieves the path of a git object
func goGetGitObjectPath(l *lua.LState) int {
	object := l.ToString(1)

	cmd := exec.Command("git", "rev-parse", object)
	output, err := cmd.CombinedOutput()
	if err != nil {
		l.Push(lua.LString(err.Error()))
		return 1
	}

	l.Push(lua.LString(string(output)))
	return 0
}

// goRegexMatch checks if a string matches a regex pattern
func goRegexMatch(l *lua.LState) int {
	pattern := l.ToString(1)
	text := l.ToString(2)

	regex, err := regexp.Compile(pattern)
	if err != nil {
		l.Push(lua.LString(fmt.Sprintf("Error: %s", err.Error())))
		return 1
	}

	matched := regex.MatchString(text)
	l.Push(lua.LBool(matched))
	return 1
}

// goRegexMatchGitDiff processes the git diff output and matches against Regex patterns
func goRegexMatchGitDiff(l *lua.LState) int {
	start := time.Now()

	output, err := getGitDiffOutput()
	if err != nil {
		l.Push(lua.LString(err.Error()))
		return 1
	}

	scanner := bufio.NewScanner(strings.NewReader(output))
	patterns := goPatternsToList(l.ToTable(1))
	results := map[string][]map[string]interface{}{}

	currentFile := ""
	lineNumber := 0
	for scanner.Scan() {
		line := scanner.Text()

		switch {
		case strings.HasPrefix(line, "+++"):
			currentFile = strings.TrimPrefix(line, "+++ ")
		case strings.HasPrefix(line, "@@"):
			parts := strings.Split(line, " ")
			if len(parts) >= 3 {
				lineNumber, _ = strconv.Atoi(strings.Trim(strings.Split(parts[2], ",")[0], "+"))
			}
		case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
			lineNumber++
			for _, pattern := range patterns {
				if pattern.Regex.MatchString(line) {
					if _, ok := results[currentFile]; !ok {
						results[currentFile] = []map[string]interface{}{}
					}
					results[currentFile] = append(results[currentFile], map[string]interface{}{
						"type":     pattern.Key,
						"line_num": lineNumber,
						"content":  line,
					})
				}
			}
		}
	}
	log.Printf("match: %v", time.Since(start))

	if err := scanner.Err(); err != nil {
		l.Push(lua.LString(err.Error()))
		return 1
	}

	// Convert results to Lua table
	luaResults := l.NewTable()
	for file, matches := range results {
		fileTable := l.NewTable()
		for _, match := range matches {
			matchTable := l.NewTable()
			matchTable.RawSetString("type", lua.LString(match["type"].(string)))
			matchTable.RawSetString("line_num", lua.LNumber(match["line_num"].(int)))
			matchTable.RawSetString("content", lua.LString(match["content"].(string)))
			fileTable.Append(matchTable)
		}
		luaResults.RawSetString(file, fileTable)
	}

	l.Push(luaResults)
	return 0
}
