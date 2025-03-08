// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

// This file contains modified code from the lua-sandbox project, available at
// https://github.com/kikito/lua-sandbox/blob/master/lua, and licensed
// under the MIT License

package luasandbox

import (
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	lua "github.com/yuin/gopher-lua"
)

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
	"regexMatch": goRegexMatch,
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
