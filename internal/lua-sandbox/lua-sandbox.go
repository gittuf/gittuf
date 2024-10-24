package lua

import (
	"context"
	"os/exec"
	"time"

	lua "github.com/yuin/gopher-lua"
)

var setupEnvironment = `
repositoryInformation = {}
repositoryInformation["user.name"] = "Jane Doe"
repositoryInformation["user.email"] = "jane.doe@example.com"
`

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

function runFunc(funcName, path, args)
	output = runCommand(funcName, path)
	print(output)
	return output
end

function goLinter(path) return runFunc("goLinter", path, nil) end
`

func NewLuaEnvironment(luaMode string) (*lua.LState, error) {
	lState := lua.NewState(lua.Options{SkipOpenLibs: true})
	modules := []struct {
		n string
		f lua.LGFunction
	}{
		{lua.LoadLibName, lua.OpenPackage}, // Must be first
		{lua.BaseLibName, lua.OpenBase},
		{lua.TabLibName, lua.OpenTable},
		{lua.StringLibName, lua.OpenString},
		{lua.MathLibName, lua.OpenMath},
	}

	switch luaMode {
	case "lessStrict":
		modules = append(modules, struct {
			n string
			f lua.LGFunction
		}{lua.OsLibName, lua.OpenOs})
		setTimeOut(context.Background(), lState, 5000)
	case "default":
		// Todo: Figure out a default mode for lua sandbox
	}

	for _, pair := range modules {
		if err := lState.CallByParam(lua.P{
			Fn:      lState.NewFunction(pair.f),
			NRet:    0,
			Protect: true,
		}, lua.LString(pair.n)); err != nil {
			panic(err)
		}
	}

	if err := lState.DoString(setupEnvironment); err != nil {
		return nil, err
	}

	lState.SetGlobal("runCommand", lState.NewFunction(runCommand))

	if err := lState.DoString(helpers); err != nil {
		return nil, err
	}

	return lState, nil
}

func setTimeOut(ctx context.Context, lState *lua.LState, timeOut int) {
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeOut)*time.Second)
	defer cancel()

	lState.SetContext(ctx)
}

// runCommand parse the function and run only supported functions
func runCommand(l *lua.LState) int {
	function := l.ToString(1)
	path := l.ToString(2)

	var command string
	var args []string

	switch function {
	case "goLinter":
		command = "golangci-lint"
		args = []string{"run", path}
	default:
		return 0
	}

	// execute command and capture output
	cmd := exec.Command(command, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		l.Push(lua.LString(err.Error()))
		return 1
	}

	// return output to lua
	l.Push(lua.LString(string(output)))
	return 1
}
