// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

// This file contains modified code from the lua- project, available at
// https://github.com/kikito/lua-/blob/master/.lua, and licensed
// under the MIT License

package luasandbox

import (
	"context"
	"fmt"
	"time"

	lua "github.com/yuin/gopher-lua"
)

// NewLuaEnvironment creates a new Lua  with the specified environment
func NewLuaEnvironment(allowedModules []string) (*lua.LState, error) {
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

	// Enable only safe functions
	enableOnlySafeFunctions(lState)

	// Set the instruction quota and timeout
	setTimeOut(context.Background(), lState, 600)
	if err := setInstructionQuota(lState, 300000000); err != nil {
		return nil, fmt.Errorf("error setting instruction quota: %w", err)
	}

	// Register the Go functions with the Lua state
	lState, err := registerAPIFunctions(lState, allowedModules)
	if err != nil {
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
	// -- * io.*, os.*: Most stuff there is unsafe

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
func setInstructionQuota(l *lua.LState, quota int64) error {
	// Run the instruction quota setting code directly
	err := l.DoString(fmt.Sprintf(`
	local count = 0
	debug.sethook(function()
		count = count + 1
		if count > %d then
			error("Instruction quota exceeded (%d instructions).", 2)
		end
	end, "", 1)
	`, quota, quota))
	if err != nil {
		return fmt.Errorf("error setting instruction quota: %w", err)
	}
	return nil
}

// Set the timeout for the Lua state
func setTimeOut(ctx context.Context, lState *lua.LState, timeOut int) {
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeOut)*time.Second)
	defer cancel()

	lState.SetContext(ctx)
}
