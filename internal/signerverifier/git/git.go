// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package git

import (
	"errors"
	"fmt"

	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/signerverifier/sigstore"
	sigstoresigneropts "github.com/gittuf/gittuf/internal/signerverifier/sigstore/options/signer"
	"github.com/gittuf/gittuf/internal/signerverifier/ssh"
	sslibdsse "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
)

var (
	ErrNoGitKeyConfigured = errors.New("no key configured in Git configuration")
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
	switch {
	case config["gpg.x509.program"] == "gitsign":
		// gitsign
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
	case config["gpg.format"] == "ssh":
		// SSH
		return ssh.NewSignerFromFile(config["user.signingkey"])
	case config["user.signingkey"] != "":
		// GPG
		return nil, fmt.Errorf("not implemented")
	default:
		return nil, ErrNoGitKeyConfigured
	}
}
