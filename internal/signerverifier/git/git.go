// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package git

import (
	"errors"

	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/signerverifier/gpg"
	"github.com/gittuf/gittuf/internal/signerverifier/sigstore"
	sigstoresigneropts "github.com/gittuf/gittuf/internal/signerverifier/sigstore/options/signer"
	"github.com/gittuf/gittuf/internal/signerverifier/ssh"
	sslibdsse "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
)

var (
	ErrUnsupportedSigningMethod = errors.New("unsupported signing method specified in Git configuration")
	ErrSigningKeyNotSpecified   = errors.New("signing key not specified in Git configuration")
	ErrUnsupportedX509Method    = errors.New("unsupported X509 certificate specified in Git configuration")
)

type SigningMethod int

const (
	SigningMethodGPG SigningMethod = iota
	SigningMethodSSH
	SigningMethodX509
)

// LoadSignerFromGitConfig loads a metadata signer from the signing key
// specified in the Git config. Currently, only SSH keys and Sigstore are
// supported.
func LoadSignerFromGitConfig(repo *gitinterface.Repository) (sslibdsse.SignerVerifier, error) {
	config, err := repo.GetGitConfig()
	if err != nil {
		return nil, err
	}

	// Attempt to determine what type of key is specified by the user's Git
	// config
	var keyType SigningMethod
	switch config["gpg.format"] {
	case "gpg", "":
		// GPG is assumed if "gpg" is specified, or if nothing is specified
		keyType = SigningMethodGPG
	case "ssh":
		keyType = SigningMethodSSH
	case "x509":
		keyType = SigningMethodX509
	default:
		// If some other format specified, return error
		return nil, ErrUnsupportedSigningMethod
	}

	// Get the path to the signing key, required if using an SSH or GPG key
	signingKey := config["user.signingkey"]
	if signingKey == "" && (keyType == SigningMethodSSH || keyType == SigningMethodGPG) {
		return nil, ErrSigningKeyNotSpecified
	}

	switch keyType {
	case SigningMethodGPG:
		// GPG
		// Load a GPG signer from the specified key
		return gpg.NewSignerFromKeyID(signingKey)
	case SigningMethodSSH:
		// SSH
		// Load an SSH signer from the specified key
		return ssh.NewSignerFromFile(signingKey)
	case SigningMethodX509:
		// X.509
		// We only support sigstore X.509, so check that gitsign is specified
		if config["gpg.x509.program"] == "gitsign" {
			// gitsign
			// Read some more configuration options and load a sigstore signer
			opts := []sigstoresigneropts.Option{}

			// Parse relevant gitsign.<> config values
			if value, has := config[sigstore.GitConfigIssuer]; has {
				opts = append(opts, sigstoresigneropts.WithIssuerURL(value))
			}
			if value, has := config[sigstore.GitConfigClientID]; has {
				opts = append(opts, sigstoresigneropts.WithClientID(value))
			}
			if value, has := config[sigstore.GitConfigFulcio]; has {
				opts = append(opts, sigstoresigneropts.WithFulcioURL(value))
			}
			if value, has := config[sigstore.GitConfigRekor]; has {
				opts = append(opts, sigstoresigneropts.WithRekorURL(value))
			}
			if value, has := config[sigstore.GitConfigRedirectURL]; has {
				opts = append(opts, sigstoresigneropts.WithRedirectURL(value))
			}
			return sigstore.NewSigner(opts...), nil
		}
		return nil, ErrUnsupportedX509Method
	default:
		return nil, ErrSigningKeyNotSpecified
	}
}
