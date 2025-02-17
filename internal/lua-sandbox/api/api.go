// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

// This file contains modified code from the lua-sandbox project, available at
// https://github.com/kikito/lua-sandbox/blob/master/sandbox.lua, and licensed
// under the MIT License

package sandbox

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	sandbox "github.com/gittuf/gittuf/internal/lua-sandbox/util"
	lua "github.com/yuin/gopher-lua"
)

// Lua sandbox helper functions and wrappers functions of provided Go APIs
var luaHelpers = `
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

function set_instruction_quota(quota)
    local count = 0

    debug.sethook(function()
        count = count + 1
        if count > quota then
            error("Instruction quota exceeded (" .. quota .. " instructions).", 2)
        end
    end, "", 1)
end

function goLinter(path) return runFunc("goLinter", path, nil) end
function removeFile(path) return deleteFile(path) end
function scanDir(path, recursive) return goScanDir(path, recursive) end
function getRootDir() return allowedDir end
function readFileAsString(fileName) return goReadFile(fileName) end
function getDiff() return goGetDiff() end
function getWorkTree() return goGetWorkTree() end
function regexMatch(text, patterns) return goRegexMatch(text, patterns) end
function getGitObject(object) return goGetGitObject(object) end
function getGitObjectSize(object) return goGetGitObjectSize(object) end
function getGitObjectHash(object) return goGetGitObjectHash(object) end
function getGitObjectPath(object) return goGetGitObjectPath(object) end
function getGitDiff() return goGetDiff() end
function getGitDiffFiles() return goGetWorkTree() end
function getGitDiffOutput() return goGetDiff() end
`

func RegisterAPIFunctions(lState *lua.LState, allowedModules []string) (*lua.LState, error) {
	lState.SetGlobal("goExecute", lState.NewFunction(goExecute))
	lState.SetGlobal("goScanDir", lState.NewFunction(goScanDir))
	lState.SetGlobal("allowedDir", lua.LString(sandbox.AllowedDir))
	lState.SetGlobal("goReadFile", lState.NewFunction(goReadFile))
	lState.SetGlobal("goRegexMatch", lState.NewFunction(goRegexMatch))
	lState.SetGlobal("goGetDiff", lState.NewFunction(goGetDiff))
	lState.SetGlobal("goGetWorkTree", lState.NewFunction(goGetWorkTree))
	lState.SetGlobal("goProcessGitDiff", lState.NewFunction(goProcessGitDiff))
	lState.SetGlobal("goCheckAddedLargeFile", lState.NewFunction(goCheckAddedLargeFile))
	lState.SetGlobal("goCheckMergeConflict", lState.NewFunction(goCheckMergeConflict))
	lState.SetGlobal("goCheckJSON", lState.NewFunction(goCheckJSON))
	lState.SetGlobal("goCheckNoCommitOnBranch", lState.NewFunction(goCheckNoCommitOnBranch))
	lState.SetGlobal("goGetGitObject", lState.NewFunction(goGetGitObject))
	lState.SetGlobal("goGetGitObjectSize", lState.NewFunction(goGetGitObjectSize))
	lState.SetGlobal("goGetGitObjectHash", lState.NewFunction(goGetGitObjectHash))
	lState.SetGlobal("goGetGitObjectPath", lState.NewFunction(goGetGitObjectPath))

	if err := lState.DoString(luaHelpers); err != nil {
		return nil, err
	}

	lState.SetGlobal("hookParameters", lua.LString(""))
	lState.SetGlobal("hookExitCode", lua.LNumber(0))
	lState.SetGlobal("allowedExecutables", lua.LString(strings.Join(allowedModules, ",")))

	return lState, nil
}

// Return all filenames in the specified directory, take a second argument to
// specify if the scan should be recursive
func goScanDir(l *lua.LState) int {
	dirPath := l.ToString(1)
	recursive := l.ToBool(2)
	if !sandbox.IsPathAllowed(dirPath) {
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

// goReadFile reads the content of a file and returns it as a string
func goReadFile(l *lua.LState) int {
	filePath := l.ToString(1)
	if !sandbox.IsPathAllowed(filePath) {
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
	cmd := exec.Command("git", "diff", "HEAD", "--no-ext-diff", "--unified=0", "-a", "--no-prefix")
	output, err := cmd.CombinedOutput()
	if err != nil {
		l.Push(lua.LString(err.Error()))
		return 1
	}

	l.Push(lua.LString(string(output)))
	return 1
}

// goGetWorkTree retrieves the list of files in the git work tree
func goGetWorkTree(l *lua.LState) int {
	cmd := exec.Command("git", "ls-files")
	output, err := cmd.CombinedOutput()

	if err != nil {
		l.Push(lua.LString(err.Error()))
		return 1
	}

	l.Push(lua.LString(string(output)))
	return 1
}

// goRegexMatch processes input text and searches for patterns
func goRegexMatch(l *lua.LState) int {
	startTime := time.Now()

	// Get input parameters from Lua state
	gitDiffOutput := l.ToString(1) // The git diff text
	patterns := l.ToTable(2)       // Table of provided regex patterns

	// Initialize tracking variables
	results := map[string][]map[string]interface{}{} // Store matches per file
	currentFile := ""                                // Track current file being processed
	lineNumber := 0                                  // Track current line number

	// Split input into lines for processing
	parseStart := time.Now()
	lines := strings.Split(gitDiffOutput, "\n")
	log.Printf("Parse: %v", time.Since(parseStart))

	// Process each line of the diff
	regexStart := time.Now()
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "+++"):
			// Found new file header, update current file being processed
			currentFile = strings.TrimPrefix(line, "+++ ")

		case strings.HasPrefix(line, "@@"):
			// Found diff hunk header, extract starting line number
			parts := strings.Split(line, " ")
			if len(parts) >= 3 {
				lineNumber, _ = strconv.Atoi(strings.Trim(strings.Split(parts[2], ",")[0], "+"))
			}

		case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
			// For each added line, check against regex patterns
			lineNumber++
			for _, pattern := range goPatternsToMap(patterns) {
				if match, _ := regexp.MatchString(pattern.Value, line); match {
					// Initialize results array for file
					if _, ok := results[currentFile]; !ok {
						results[currentFile] = []map[string]interface{}{}
					}
					// Record the match details
					results[currentFile] = append(results[currentFile], map[string]interface{}{
						"type":     pattern.Key,
						"line_num": lineNumber,
						"content":  line,
					})
				}
			}
		case strings.HasPrefix(line, " "):
			// Increment line counter
			lineNumber++
		}
	}
	log.Printf("Match: %v", time.Since(regexStart))

	// Convert results to Lua tables
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

	// Return results to Lua
	l.Push(luaResults)
	log.Printf("Total time: %v", time.Since(startTime))
	return 1
}

// goExecute parse the function and run only supported functions
func goExecute(l *lua.LState) int {
	command := l.ToString(1)
	commands := strings.Split(command, " ")

	// TODO: Verify the integrity of the executable
	var executable string
	var args []string

	// TODO: Make the args parsing more robust
	executable = commands[0]
	args = commands[1:]

	if !strings.Contains(l.GetGlobal("allowedExecutables").String(), executable) {
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

// goPatternsToMap converts Lua table of patterns to Go struct
func goPatternsToMap(patterns *lua.LTable) []struct{ Key, Value string } {
	result := []struct{ Key, Value string }{}
	patterns.ForEach(func(key lua.LValue, value lua.LValue) {
		result = append(result, struct{ Key, Value string }{
			Key:   key.String(),
			Value: value.String(),
		})
	})
	return result
}

type RegexPattern struct {
	Key   string
	Regex *regexp.Regexp
}

// goPatternsToList converts Lua table of patterns to Go struct
func goPatternsToList(patterns *lua.LTable) []RegexPattern {
	var patternsList []RegexPattern
	patterns.ForEach(func(key, value lua.LValue) {
		patternsList = append(patternsList, RegexPattern{
			Key:   key.String(),
			Regex: regexp.MustCompile(value.String()),
		})
	})
	return patternsList
}

// goProcessGitDiff processes the git diff output and matches against patterns
func goProcessGitDiff(l *lua.LState) int {
	start := time.Now()

	output, err := sandbox.GetGitDiffOutput()
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

// goCheckAddedLargeFile checks for large files in the git diff output
func goCheckAddedLargeFile(l *lua.LState) int {
	maxSizeKB := l.ToInt(1)
	enforceAll := l.ToBool(2)

	var files []string
	var err error

	if enforceAll {
		files, err = sandbox.GetWorkTreeFiles()
	} else {
		files, err = sandbox.GetGitDiffFiles()
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

	files, err = sandbox.GetGitDiffFiles()

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

	files, err = sandbox.GetWorkTreeFiles()

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

	branchName, err := sandbox.GetCurrentBranchName()
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
