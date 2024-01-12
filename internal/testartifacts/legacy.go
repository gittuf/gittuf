// SPDX-License-Identifier: Apache-2.0

package artifacts

import _ "embed"

// NOTE: This on-disk key format has been retired, but we're keeping it around
// for compatibility. These will eventually be removed.

//go:embed testdata/keys/legacy/1.pub
var SSLibKey1Public []byte

//go:embed testdata/keys/legacy/1
var SSLibKey1Private []byte

//go:embed testdata/keys/legacy/2.pub
var SSLibKey2Public []byte

//go:embed testdata/keys/legacy/2
var SSLibKey2Private []byte

//go:embed testdata/keys/legacy/3.pub
var SSLibKey3Public []byte

//go:embed testdata/keys/legacy/3
var SSLibKey3Private []byte
