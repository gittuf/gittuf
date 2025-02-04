// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package removeperson

import (
	"fmt"

	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/cmd/common"
	"github.com/gittuf/gittuf/internal/cmd/policy/persistent"
	"github.com/gittuf/gittuf/internal/dev"
	"github.com/gittuf/gittuf/internal/policy"
	tufv02 "github.com/gittuf/gittuf/internal/tuf/v02"
	"github.com/spf13/cobra"
)

type options struct {
	p          *persistent.Options
	policyName string
	personID   string
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(
		&o.policyName,
		"policy-name",
		policy.TargetsRoleName,
		"name of policy file to remove person from",
	)

	cmd.Flags().StringVar(
		&o.personID,
		"person-ID",
		"",
		"person ID",
	)
	cmd.MarkFlagRequired("person-ID") //nolint:errcheck
}

func (o *options) Run(cmd *cobra.Command, _ []string) error {
	if !tufv02.AllowV02Metadata() {
		return fmt.Errorf("developer mode and v0.2 policy metadata must be enabled, set %s=1 and %s=1", dev.DevModeKey, tufv02.AllowV02MetadataKey)
	}

	repo, err := gittuf.LoadRepository()
	if err != nil {
		return err
	}

	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}

	return repo.RemovePrincipalFromTargets(cmd.Context(), signer, o.policyName, o.personID, true)
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:               "remove-person",
		Short:             fmt.Sprintf("Remove a person from a policy file (requires developer mode and v0.2 policy metadata to be enabled, set %s=1 and %s=1)", dev.DevModeKey, tufv02.AllowV02MetadataKey),
		Long:              `This command allows users to remove a person from the specified policy file. The person's ID is required. By default, the main policy file is selected.`,
		PreRunE:           common.CheckForSigningKeyFlag,
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
