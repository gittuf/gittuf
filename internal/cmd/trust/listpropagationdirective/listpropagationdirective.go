// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package listpropagationdirective

import (
	"fmt"
	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/cmd/common"
	"github.com/gittuf/gittuf/internal/cmd/trust/persistent"
	"github.com/spf13/cobra"

)

type options struct {
	p                   *persistent.Options
}

func (o *options) AddFlags(cmd *cobra.Command) {
}

func (o *options) Run(cmd *cobra.Command, _ []string) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}

	directives, err := repo.GetPropagationDirectives(cmd.Context(), signer, true)
	if err != nil {
		return err
	}
	fmt.Printf("Propagation Directives in the gittuf root of trust:\n")
	for directive, pd := range directives {
		fmt.Printf("%d: %v\n", directive, pd)
	}

	return nil 
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:               "list-propagation-directive",
		Short:             "Lists propagation directives in the gittuf root of trust",
		PreRunE:           common.CheckForSigningKeyFlag,
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
