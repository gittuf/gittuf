// SPDX-License-Identifier: Apache-2.0

package artifacts

import _ "embed"

//go:embed testdata/keys/ssh/rsa.pem
var SSHRSAPublic []byte

//go:embed testdata/keys/ssh/rsa.pub
var SSHRSAPublicSSH []byte

//go:embed testdata/keys/ssh/rsa
var SSHRSAPrivate []byte

//go:embed testdata/keys/ssh/rsa_enc
var SSHRSAPrivateEnc []byte

//go:embed testdata/keys/ssh/ecdsa.pem
var SSHECDSAPublic []byte

//go:embed testdata/keys/ssh/ecdsa.pub
var SSHECDSAPublicSSH []byte

//go:embed testdata/keys/ssh/ecdsa
var SSHECDSAPrivate []byte

//go:embed testdata/keys/ssh/ed25519.pem
var SSHED25519Public []byte

//go:embed testdata/keys/ssh/ed25519.pub
var SSHED25519PublicSSH []byte

//go:embed testdata/keys/ssh/ed25519
var SSHED25519Private []byte
