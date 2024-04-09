// SPDX-License-Identifier: Apache-2.0

package artifacts

import _ "embed"

//go:embed testdata/gitconfigs/config-1
var GitConfig1 []byte

//go:embed testdata/gitconfigs/config-2
var GitConfig2 []byte

//go:embed testdata/gitconfigs/config-3
var GitConfig3 []byte

//go:embed testdata/gitconfigs/config-4
var GitConfig4 []byte
