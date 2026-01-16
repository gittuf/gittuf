// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gittuf

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/gittuf/gittuf/internal/signerverifier/gpg"
	"github.com/gittuf/gittuf/internal/signerverifier/sigstore"
	sigstoresigneropts "github.com/gittuf/gittuf/internal/signerverifier/sigstore/options/signer"
	"github.com/gittuf/gittuf/internal/signerverifier/ssh"
	sslibdsse "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
	"github.com/gittuf/gittuf/internal/tuf"
	tufv01 "github.com/gittuf/gittuf/internal/tuf/v01"
	"github.com/secure-systems-lab/go-securesystemslib/signerverifier"
)

const (
	GPGKeyPrefix = "gpg:"
	FulcioPrefix = "fulcio:"
)

type signingMethod int

const (
	signingMethodGPG signingMethod = iota
	signingMethodSSH
	signingMethodX509
)

var (
	ErrUnsupportedSigningMethod = errors.New("unsupported signing method specified in Git configuration")
	ErrSigningKeyNotSpecified   = errors.New("signing key not specified in Git configuration")
	ErrUnsupportedX509Method    = errors.New("unsupported X509 certificate specified in Git configuration")
)

// LoadPublicKey returns a signerverifier.SSLibKey object for a PGP / Sigstore
// Fulcio / SSH (on-disk) key for use in gittuf metadata.
func LoadPublicKey(keyRef string) (tuf.Principal, error) {
	var (
		keyObj *signerverifier.SSLibKey
		err    error
	)

	switch {
	case strings.HasPrefix(keyRef, GPGKeyPrefix):
		fingerprint := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(keyRef, GPGKeyPrefix)))

		command := exec.Command("gpg", "--export", "--armor", fingerprint)
		stdOut, err := command.Output()
		if err != nil {
			return nil, err
		}

		keyObj, err = gpg.LoadGPGKeyFromBytes(stdOut)
		if err != nil {
			return nil, err
		}
	case strings.HasPrefix(keyRef, FulcioPrefix):
		keyID := strings.TrimPrefix(keyRef, FulcioPrefix)
		ks := strings.Split(keyID, "::")
		if len(ks) != 2 {
			return nil, fmt.Errorf("incorrect format for fulcio identity")
		}

		keyObj = &signerverifier.SSLibKey{
			KeyID:   keyID,
			KeyType: sigstore.KeyType,
			Scheme:  sigstore.KeyScheme,
			KeyVal: signerverifier.KeyVal{
				Identity: ks[0],
				Issuer:   ks[1],
			},
		}
	default:
		keyObj, err = ssh.NewKeyFromFile(keyRef)
		if err != nil {
			return nil, err
		}
	}

	return tufv01.NewKeyFromSSLibKey(keyObj), nil
}

// LoadSigner loads a metadata signer for the specified key bytes. The signer
// must be for a GPG key (in which case the `key` is the GPG key ID), an SSH key
// (in which case the `key` is a path to the private key) or for signing with
// Sigstore (where `key` has a prefix `fulcio:`). If no key ID is specified,
// this function calls LoadSignerFromGitConfig.
func LoadSigner(repo *Repository, key string) (sslibdsse.SignerVerifier, error) {
	if key == "" {
		return LoadSignerFromGitConfig(repo)
	}

	switch {
	case strings.HasPrefix(key, GPGKeyPrefix):
		keyID := strings.TrimPrefix(key, GPGKeyPrefix)

		gitRepo := repo.GetGitRepository()
		config, err := gitRepo.GetGitConfig()
		if err != nil {
			return nil, err
		}

		return gpg.NewSignerFromKeyID(keyID, getGPGOptions(config)...)
	case strings.HasPrefix(key, FulcioPrefix):
		gitRepo := repo.GetGitRepository()
		config, err := gitRepo.GetGitConfig()
		if err != nil {
			return nil, err
		}

		return sigstore.NewSigner(getSigstoreOptions(config)...), nil
	default:
		return ssh.NewSignerFromFile(key)
	}
}

// LoadSignerFromGitConfig loads a metadata signer for the signing key specified
// in the Git configuration of the target repository.
func LoadSignerFromGitConfig(repo *Repository) (sslibdsse.SignerVerifier, error) {
	config, err := repo.r.GetGitConfig()
	if err != nil {
		return nil, err
	}

	// Attempt to determine what type of key is specified by the user's Git
	// config
	var keyType signingMethod
	switch config["gpg.format"] {
	case "gpg", "":
		// GPG is assumed if "gpg" is specified, or if nothing is specified
		keyType = signingMethodGPG
	case "ssh":
		keyType = signingMethodSSH
	case "x509":
		keyType = signingMethodX509
	default:
		// If some other format specified, return error
		return nil, ErrUnsupportedSigningMethod
	}

	// Get the path to the signing key, required if using an SSH or GPG key
	signingKey := config["user.signingkey"]
	if signingKey == "" && (keyType == signingMethodSSH || keyType == signingMethodGPG) {
		return nil, ErrSigningKeyNotSpecified
	}

	switch keyType {
	case signingMethodGPG:
		// GPG
		// Load a GPG signer from the specified key
		return gpg.NewSignerFromKeyID(signingKey, getGPGOptions(config)...)
	case signingMethodSSH:
		// SSH
		// Load an SSH signer from the specified key
		return ssh.NewSignerFromFile(signingKey)
	case signingMethodX509:
		// X.509
		// We only support sigstore X.509, so check that gitsign is specified
		if config["gpg.x509.program"] == "gitsign" {
			// gitsign
			return sigstore.NewSigner(getSigstoreOptions(config)...), nil
		}
		return nil, ErrUnsupportedX509Method
	default:
		return nil, ErrSigningKeyNotSpecified
	}
}

func getGPGOptions(config map[string]string) []gpg.SignerOption {
	opts := []gpg.SignerOption{}
	// Parse relevant gpg.<name> config values
	if value, has := config["gpg.program"]; has {
		opts = append(opts, gpg.WithGPGProgram(value))
	}
	return opts
}

func getSigstoreOptions(config map[string]string) []sigstoresigneropts.Option {
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
	return opts
}
