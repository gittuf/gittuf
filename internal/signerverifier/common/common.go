// SPDX-License-Identifier: Apache-2.0

package common

import "errors"

var (
	ErrSignatureVerificationFailed = errors.New("failed to verify signature")
	ErrNotPrivateKey               = errors.New("loaded key is not a private key")
	ErrUnknownKeyType              = errors.New("unknown key type")
	ErrInvalidThreshold            = errors.New("threshold is either less than 1 or greater than number of provided public keys")
)
