// SPDX-License-Identifier: Apache-2.0

package artifacts

import _ "embed"

//go:embed testdata/keys/gpg/1.pub.asc
var GPGKey1Public []byte

//go:embed testdata/keys/gpg/1.asc
var GPGKey1Private []byte

//go:embed testdata/keys/gpg/2.pub.asc
var GPGKey2Public []byte

//go:embed testdata/keys/gpg/2.asc
var GPGKey2Private []byte

//go:embed testdata/configurations/gpg-agent.conf
var GPGAgentConf []byte
