// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package common //nolint:revive

import (
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/sigstore/sigstore/pkg/cryptoutils"
)

var (
	ErrSignatureVerificationFailed = errors.New("failed to verify signature")
	ErrNotPrivateKey               = errors.New("loaded key is not a private key")
	ErrUnknownKeyType              = errors.New("unknown key type")
	ErrInvalidThreshold            = errors.New("threshold is either less than 1 or greater than number of provided public keys")
)

// LoadCertsFromPath opens the file at the specified path and parses the
// certificates present in PEM form. This is similar to a helper in
// https://github.com/sigstore/sigstore and is used in gittuf's sigstore signing
// and verification flows.
func LoadCertsFromPath(path string) ([]*x509.Certificate, error) {
	slog.Debug(fmt.Sprintf("Loading %s...", path))
	pemBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	certs, err := cryptoutils.UnmarshalCertificatesFromPEM(pemBytes)
	if err != nil {
		return nil, err
	}

	if len(certs) == 0 {
		return nil, fmt.Errorf("no certificates in file %s", path)
	}

	return certs, nil
}
