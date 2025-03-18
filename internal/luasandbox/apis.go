// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package luasandbox

import (
	"fmt"
	"regexp"

	lua "github.com/yuin/gopher-lua"
)

// API presents the interface for any API made available within the sandbox.
type API interface {
	GetName() string
	GetSignature() string
	GetHelp() string
	GetExamples() []string
}

// LuaAPI implements the API interface. This is used when the API is implemented
// as a Lua function.
type LuaAPI struct {
	Name           string
	Signature      string
	Help           string
	Examples       []string
	Implementation string
}

func (l *LuaAPI) GetName() string {
	return l.Name
}

func (l *LuaAPI) GetSignature() string {
	return l.Signature
}

func (l *LuaAPI) GetHelp() string {
	return l.Help
}

func (l *LuaAPI) GetExamples() []string {
	return l.Examples
}

// GoAPI implements the API interface. This is used when the API is implemented
// in Go.
type GoAPI struct {
	Name           string
	Signature      string
	Help           string
	Examples       []string
	Implementation lua.LGFunction
}

func (g *GoAPI) GetName() string {
	return g.Name
}

func (g *GoAPI) GetSignature() string {
	return g.Signature
}

func (g *GoAPI) GetHelp() string {
	return g.Help
}

func (g *GoAPI) GetExamples() []string {
	return g.Examples
}

func (l *LuaEnvironment) apiMatchRegex() API {
	return &GoAPI{
		Name:      "matchRegex",
		Signature: "matchRegex(pattern, text) -> matched",
		Help:      "Check if the regular expression pattern matches the provided text.",
		Implementation: func(l *lua.LState) int {
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
		},
	}
}

func (l *LuaEnvironment) apiStrSplit() API {
	return &LuaAPI{
		Name:      "strSplit",
		Signature: "strSplit(str, sep) -> components",
		Help:      "Split string using provided separator. If a separator is not provided, then \"\\n\" is used by default.",
		// TODO: check if examples are right
		Examples: []string{
			"strSplit(\"hello\\nworld\") -> [\"hello\", \"world\"]",
			"strSplit(\"hello\\nworld\", \"\\n\") -> [\"hello\", \"world\"]",
		},
		Implementation: `
		function strSplit(str, sep)
			if sep == nil then
				sep = "\n"
			end
			local components = {}
			for component in string.gmatch(str, "([^"..sep.."]+)") do
				table.insert(components, component)
			end
			return components
		end	
		`,
	}
}
