// SPDX-License-Identifier: Apache-2.0

package artifacts

import _ "embed"

//go:embed testdata/keys/ssh/rsa.pem
var SSHRSAPublic []byte

//go:embed testdata/keys/ssh/rsa
var SSHRSAPrivate []byte

//go:embed testdata/keys/ssh/ecdsa.pem
var SSHECDSAPublic []byte

//go:embed testdata/keys/ssh/ecdsa
var SSHECDSAPrivate []byte

// TODO: ED25519 will be supported after
// https://bugzilla.mindrot.org/show_bug.cgi?id=3195.
// var eD25519Public []byte
// var eD25519Private []byte
