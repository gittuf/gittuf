// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tuf

import "testing"

func FuzzHookStageUnmarshalText(f *testing.F) {
	f.Add([]byte(HookStagePreCommitString))
	f.Add([]byte(HookStagePrePushString))
	f.Add([]byte(""))
	f.Add([]byte("invalid"))

	f.Fuzz(func(_ *testing.T, text []byte) {
		var h HookStage
		_ = h.UnmarshalText(text)
	})
}

func FuzzHookStageUnmarshalJSON(f *testing.F) {
	f.Add([]byte(`"` + HookStagePreCommitString + `"`))
	f.Add([]byte(`"` + HookStagePrePushString + `"`))
	f.Add([]byte(`"invalid"`))
	f.Add([]byte(`123`))
	f.Add([]byte(``))
	f.Add([]byte(`null`))

	f.Fuzz(func(_ *testing.T, data []byte) {
		var h HookStage
		_ = h.UnmarshalJSON(data)
	})
}

func FuzzHookEnvironmentUnmarshalJSON(f *testing.F) {
	f.Add([]byte(`"` + HookEnvironmentLuaString + `"`))
	f.Add([]byte(`"invalid"`))
	f.Add([]byte(`123`))
	f.Add([]byte(``))
	f.Add([]byte(`null`))

	f.Fuzz(func(_ *testing.T, data []byte) {
		var h HookEnvironment
		_ = h.UnmarshalJSON(data)
	})
}
