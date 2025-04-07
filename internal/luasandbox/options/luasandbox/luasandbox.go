// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package luasandbox

type EnivronmentOptions struct {
	LuaTimeout int
}

type EnvironmentOption func(*EnivronmentOptions)

func WithLuaTimeout(timeout int) EnvironmentOption {
	return func(o *EnivronmentOptions) {
		o.LuaTimeout = timeout
	}
}
