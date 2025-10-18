package authorizewithhat

import (
	"fmt"

	"github.com/gittuf/gittuf/experimental/gittuf"
	attestopts "github.com/gittuf/gittuf/experimental/gittuf/options/attest"
	"github.com/gittuf/gittuf/internal/cmd/attest/persistent"
	"github.com/gittuf/gittuf/internal/cmd/common"
	"github.com/spf13/cobra"
)

type options struct {
	p       *persistent.Options
	fromRef string
	revoke  bool
	hat     string
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(
		&o.fromRef,
		"from-ref",
		"f",
		"",
		"ref to authorize merging changes from",
	)
	cmd.MarkFlagRequired("from-ref") //nolint:errcheck

	cmd.Flags().BoolVarP(
		&o.revoke,
		"revoke",
		"r",
		false,
		"revoke existing authorization",
	)

	cmd.Flags().StringVarP(
		&o.hat,
		"hat",
		"h",
		"",
		"hat worn while attesting",
	)
	cmd.MarkFlagRequired("hat") //nolint:errcheck
}

func (o *options) Run(cmd *cobra.Command, args []string) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}

	// TODO: is the specified hat valid for the signer? -> might be better to check during verification

	// TODO: need a hat authorization equivalent
	if o.revoke {
		if len(args) < 3 {
			return fmt.Errorf("insufficient parameters for revoking authorization, requires <targetRef> <fromID> <targetTreeID>")
		}

		return repo.RemoveReferenceAuthorization(cmd.Context(), signer, args[0], args[1], args[2], true)
	}

	opts := []attestopts.Option{}
	if o.p.WithRSLEntry {
		opts = append(opts, attestopts.WithRSLEntry())
	}

	return repo.AddReferenceAuthorizationWithHat(cmd.Context(), signer, args[0], o.fromRef, o.hat, true, opts...)
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:               "authorize",
		Short:             "Add or revoke reference authorization",
		Args:              cobra.MinimumNArgs(1),
		PreRunE:           common.CheckForSigningKeyFlag,
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
