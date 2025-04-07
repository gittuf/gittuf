// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package luasandbox

type EnvironmentOptions struct {
	LuaTimeout int
}

type EnvironmentOption func(*EnvironmentOptions)

func WithLuaTimeout(timeout int) EnvironmentOption {
	return func(o *EnvironmentOptions) {
		o.LuaTimeout = timeout
	}
}
