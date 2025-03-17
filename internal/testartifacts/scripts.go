// SPDX-License-Identifier: Apache-2.0

package artifacts

import _ "embed"

//go:embed testdata/scripts/askpass.sh
var AskpassScript []byte

//go:embed testdata/scripts/hello.lua
var SampleHookScript []byte
