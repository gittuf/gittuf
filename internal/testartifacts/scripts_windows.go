//go:build windows

// SPDX-License-Identifier: Apache-2.0

package artifacts

import _ "embed"

//go:embed testdata/scripts/askpass.ps1
var AskpassScript []byte
