// SPDX-License-Identifier: Apache-2.0

package updateruleteams

import (
	"encoding/json"
	"os"

	"github.com/gittuf/gittuf/internal/cmd/common"
	"github.com/gittuf/gittuf/internal/cmd/policy/persistent"
	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/repository"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/spf13/cobra"
)

type options struct {
	p                *persistent.Options
	policyName       string
	ruleName         string
	teamsDefinitions string
	rulePatterns     []string
	minRoles         int
}

type RoleIntermediary struct {
	Name      string   `json:"name"`
	Keys      []string `json:"keys"`
	Threshold int      `json:"threshold"`
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(
		&o.policyName,
		"policy-name",
		policy.TargetsRoleName,
		"name of policy file to add rule to",
	)

	cmd.Flags().StringVar(
		&o.ruleName,
		"rule-name",
		"",
		"name of rule",
	)
	cmd.MarkFlagRequired("rule-name") //nolint:errcheck

	cmd.Flags().StringVar(
		&o.teamsDefinitions,
		"roles-json",
		"",
		"path to role definition file",
	)
	cmd.MarkFlagRequired("roles-json") //nolint:errcheck

	cmd.Flags().StringArrayVar(
		&o.rulePatterns,
		"rule-pattern",
		[]string{},
		"patterns used to identify namespaces rule applies to",
	)
	cmd.MarkFlagRequired("rule-pattern") //nolint:errcheck

	cmd.Flags().IntVar(
		&o.minRoles,
		"min-roles",
		1,
		"minimum roles needed to agree",
	)
	cmd.MarkFlagRequired("min-roles") //nolint:errcheck
}

func (o *options) Run(cmd *cobra.Command, _ []string) error {
	repo, err := repository.LoadRepository()
	if err != nil {
		return err
	}

	keyBytes, err := os.ReadFile(o.p.SigningKey)
	if err != nil {
		return err
	}
	signer, err := common.LoadSigner(keyBytes)
	if err != nil {
		return err
	}

	roleBytes, err := os.ReadFile(o.teamsDefinitions)
	if err != nil {
		return err
	}

	rolesToBeProcessed := []RoleIntermediary{}

	err = json.Unmarshal(roleBytes, &rolesToBeProcessed)
	if err != nil {
		return err
	}

	roles := []tuf.Role{}
	authorizedKeys := []*tuf.Key{}

	for _, roleDef := range rolesToBeProcessed {
		role := tuf.Role{
			Name:      roleDef.Name,
			KeyIDs:    []string{},
			Threshold: roleDef.Threshold,
		}
		authorizedKeyStrings := []string{}
		for _, key := range roleDef.Keys {
			key, err := common.LoadPublicKey(key)
			if err != nil {
				return err
			}
			authorizedKeys = append(authorizedKeys, key)
			authorizedKeyStrings = append(authorizedKeyStrings, key.KeyID)
		}

		role.KeyIDs = authorizedKeyStrings
		roles = append(roles, role)
	}

	return repo.AddDelegation(cmd.Context(), signer, o.policyName, o.ruleName, authorizedKeys, o.rulePatterns, o.minRoles, roles, true)
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:               "update-rule",
		Short:             "Update an existing rule in a policy file",
		Long:              `This command allows users to update an existing rule to the specified policy file using the teams functionality. By default, the main policy file is selected. Roles may be specified as a JSON file. See the "teams-format.md" document in the docs directory for more information.`,
		PreRunE:           common.CheckIfSigningViableWithFlag,
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
