
package updateteam

import (
	"fmt"
	"strings"

	"github.com/gittuf/gittuf/experimental/gittuf"
	trustpolicyopts "github.com/gittuf/gittuf/experimental/gittuf/options/trustpolicy"
	"github.com/gittuf/gittuf/internal/cmd/common"
	"github.com/gittuf/gittuf/internal/cmd/policy/persistent"
	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/spf13/cobra"
)

type options struct {
	p                *persistent.Options
	policyName       string
	teamID           string
	addPersonIDs     []string
	removePersonIDs  []string
	threshold        int
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(
		&o.policyName,
		"policy-name",
		policy.TargetsRoleName,
		"name of policy file to update (default: targets)",
	)

	cmd.Flags().StringVar(
		&o.teamID,
		"team-ID",
		"",
		"team ID to update",
	)
	cmd.MarkFlagRequired("team-ID") //nolint:errcheck

	cmd.Flags().StringArrayVar(
		&o.addPersonIDs,
		"add-person",
		[]string{},
		"principal IDs to add to the team",
	)

	cmd.Flags().StringArrayVar(
		&o.removePersonIDs,
		"remove-person",
		[]string{},
		"principal IDs to remove from the team",
	)

	cmd.Flags().IntVar(
		&o.threshold,
		"threshold",
		0,
		"threshold of required valid signatures for this team",
	)
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

	opts := []trustpolicyopts.Option{}
	if o.p.WithRSLEntry {
		opts = append(opts, trustpolicyopts.WithRSLEntry())
	}

	// Fetch existing principals
	existingPrincipals, err := repo.ListPrincipals(cmd.Context(), "policy", o.policyName)
	if err != nil {
		return fmt.Errorf("failed to list principals: %w", err)
	}

	current := map[string]struct{}{}
	for _, principal := range existingPrincipals {
		custom := principal.CustomMetadata()
		if role, ok := custom["delegation-role"]; ok && role == o.teamID {
			current[principal.ID()] = struct{}{}
		}
	}

	// Remove specified personIDs
	for _, id := range o.removePersonIDs {
		delete(current, id)
	}

	// Add specified personIDs
	for _, id := range o.addPersonIDs {
		current[id] = struct{}{}
	}

	// Convert to slice
	final := []string{}
	for id := range current {
		final = append(final, id)
	}

	// Update the delegation
	err = repo.UpdateDelegation(cmd.Context(), signer, o.policyName, o.teamID, final, nil, o.threshold, true, opts...)
	if err != nil {
		return fmt.Errorf("failed to update team delegation: %w", err)
	}

	return nil
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:               "update-team",
		Short:             "Update a team's members and signature threshold",
		Long:              `The 'update-team' command updates a team's members by adding or removing specified principal IDs and updates the threshold value in a gittuf policy file.`,
		PreRunE:           common.CheckForSigningKeyFlag,
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)
	return cmd
}
