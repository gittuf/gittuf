// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

// This file contains modified code from the lua-sandbox project, available at
// https://github.com/kikito/lua-sandbox/blob/master/sandbox.lua, and licensed
// under the MIT License

package luasandbox

import (
	"context"
	"errors"
	"fmt"
	"time"

	lua "github.com/yuin/gopher-lua"
)

const (
	// luaInstructionQuota is from
	// https://mozilla-services.github.io/lua_sandbox/sandbox.html
	luaInstructionQuota = 1000000
	luaTimeOut          = 100
)

var (
	ErrMismatchedAPINames = errors.New("name of API to be registered does not match API implementation")
)

type LuaEnvironment struct {
	lState *lua.LState
}

// NewLuaEnvironment creates a new Lua state with the specified modules
// enabled.
func NewLuaEnvironment(ctx context.Context) (*LuaEnvironment, error) {
	// Create a new Lua state
	lState := lua.NewState(lua.Options{SkipOpenLibs: true})
	environment := &LuaEnvironment{lState: lState}

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
	environment.enableOnlySafeFunctions()

	// Set the instruction quota and timeout
	environment.setTimeOut(ctx, luaTimeOut)
	if err := environment.setInstructionQuota(luaInstructionQuota); err != nil {
		return nil, fmt.Errorf("error setting instruction quota: %w", err)
	}

	// Register the Go functions with the Lua state
	if err := environment.registerAPIFunctions(); err != nil {
		return nil, err
	}

	return environment, nil
}

// enableOnlySafeFunctions disables all functions that are deemed to be unsafe.
func (l *LuaEnvironment) enableOnlySafeFunctions() {
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
	// -- * debug.*: Unsafe, see https://www.lua.org/pil/23.html
	// -- * package.*: Allows arbitrary module loading, see https://www.lua.org/manual/5.3/manual.html#pdf-package

	// Disable all unsafe functions
	l.lState.SetGlobal("dofile", lua.LNil)
	l.lState.SetGlobal("load", lua.LNil)
	l.lState.SetGlobal("loadfile", lua.LNil)
	l.lState.SetGlobal("loadstring", lua.LNil)
	l.lState.SetGlobal("require", lua.LNil)
	l.lState.SetGlobal("module", lua.LNil)
	l.lState.SetGlobal("collectgarbage", lua.LNil)
	l.lState.SetGlobal("rawget", lua.LNil)
	l.lState.SetGlobal("rawset", lua.LNil)
	l.lState.SetGlobal("rawequal", lua.LNil)
	l.lState.SetGlobal("setmetatable", lua.LNil)
	l.lState.SetGlobal("getmetatable", lua.LNil)
	l.lState.SetGlobal("_G", lua.LNil)
	l.lState.SetGlobal("os", lua.LNil)
	l.lState.SetGlobal("io", lua.LNil)
	l.lState.SetGlobal("debug", lua.LNil)
	l.lState.SetGlobal("package", lua.LNil)

	if strMod, ok := l.lState.GetGlobal(lua.StringLibName).(*lua.LTable); ok {
		strMod.RawSetString("rep", lua.LNil)
		strMod.RawSetString("dump", lua.LNil)
		l.protectModule(strMod, lua.StringLibName)
	}

	// Load protected modules with only safe functions
	if mathMod, ok := l.lState.GetGlobal(lua.MathLibName).(*lua.LTable); ok {
		mathMod.RawSetString("randomseed", lua.LNil)
		l.protectModule(mathMod, lua.MathLibName)
	}

	if coroMod, ok := l.lState.GetGlobal(lua.CoroutineLibName).(*lua.LTable); ok {
		l.protectModule(coroMod, lua.CoroutineLibName)
	}

	if tabMod, ok := l.lState.GetGlobal(lua.TabLibName).(*lua.LTable); ok {
		l.protectModule(tabMod, lua.TabLibName)
	}

	if baseMod, ok := l.lState.GetGlobal(lua.BaseLibName).(*lua.LTable); ok {
		l.protectModule(baseMod, lua.BaseLibName)
	}
}

// protectModule protects the specified module from being modified by setting a
// protected metatable with __newindex and __metatable fields.
func (l *LuaEnvironment) protectModule(tbl *lua.LTable, moduleName string) {
	mt := l.lState.NewTable()
	l.lState.SetMetatable(tbl, mt)
	l.lState.SetField(mt, "__newindex", l.lState.NewFunction(func(l *lua.LState) int {
		varName := l.ToString(2)
		l.RaiseError("attempt to modify read-only table '%s.%s'", moduleName, varName)
		return 0
	}))
	l.lState.SetField(mt, "__metatable", lua.LString("protected"))
}

// setInstructionQuota sets the instruction quota for the Lua state.
func (l *LuaEnvironment) setInstructionQuota(quota int64) error {
	// Run the instruction quota setting code directly
	err := l.lState.DoString(fmt.Sprintf(`
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

// setTimeOut sets the timeout for the Lua state.
func (l *LuaEnvironment) setTimeOut(ctx context.Context, timeOut int) {
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeOut)*time.Second)
	defer cancel()

	l.lState.SetContext(ctx)
}

// registerAPIFunctions makes the sandbox's standard APIs available.
func (l *LuaEnvironment) registerAPIFunctions() error {
	// Set global variables for the Lua state
	l.lState.SetGlobal("hookParameters", lua.LString(""))
	l.lState.SetGlobal("hookExitCode", lua.LNumber(0))

	for name, availableAPI := range RegisterAPIs {
		if name != availableAPI.GetName() {
			return fmt.Errorf("%w: '%s' does not match '%s'", ErrMismatchedAPINames, name, availableAPI.GetName())
		}

		switch availableAPI := availableAPI.(type) {
		case *LuaAPI:
			if err := l.lState.DoString(availableAPI.Implementation); err != nil {
				return fmt.Errorf("unable to register API '%s': %w", name, err)
			}
		case *GoAPI:
			l.lState.SetGlobal(name, l.lState.NewFunction(availableAPI.Implementation))
		}
	}

	return nil
}
