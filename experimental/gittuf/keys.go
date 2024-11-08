// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gittuf

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/gittuf/gittuf/internal/dev"
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

// LoadSigner loads a metadata signer for the specified key bytes. Currently,
// the signer must be either for an SSH key (in which case the `key` is a path
// to the private key) or for signing with Sigstore (where `key` has a prefix
// `fulcio:`). For Sigstore, developer mode must be enabled by setting
// GITTUF_DEV=1 in the environment.
func LoadSigner(repo *Repository, key string) (sslibdsse.SignerVerifier, error) {
	switch {
	case strings.HasPrefix(key, GPGKeyPrefix):
		return nil, fmt.Errorf("not implemented")
	case strings.HasPrefix(key, FulcioPrefix):
		if !dev.InDevMode() {
			return nil, dev.ErrNotInDevMode
		}

		opts := []sigstoresigneropts.Option{}

		gitRepo := repo.GetGitRepository()
		config, err := gitRepo.GetGitConfig()
		if err != nil {
			return nil, err
		}

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
	default:
		return ssh.NewSignerFromFile(key)
	}
}
