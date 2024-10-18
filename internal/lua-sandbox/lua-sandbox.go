package lua

import lua "github.com/yuin/gopher-lua"

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
`

func NewLuaEnvironment() (*lua.LState, error) {
	lState := lua.NewState(lua.Options{SkipOpenLibs: true})
	for _, pair := range []struct {
		n string
		f lua.LGFunction
	}{
		{lua.LoadLibName, lua.OpenPackage}, // Must be first
		{lua.BaseLibName, lua.OpenBase},
		{lua.TabLibName, lua.OpenTable},
		{lua.StringLibName, lua.OpenString},
		{lua.MathLibName, lua.OpenMath},
	} {
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

	if err := lState.DoString(helpers); err != nil {
		return nil, err
	}

	return lState, nil
}
