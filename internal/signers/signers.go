package signers

import "errors"

var (
	ErrSignatureVerificationFailed = errors.New("failed to verify signature")
	ErrNotPrivateKey               = errors.New("loaded key is not a private key")
	ErrInvalidThreshold            = errors.New("threshold is either less than 1 or greater than number of provided public keys")
)
