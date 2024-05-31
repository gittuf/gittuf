// SPDX-License-Identifier: Apache-2.0

package addruleteams

import (
	"encoding/json"
	"io"
	"os"

	"github.com/gittuf/gittuf/internal/cmd/common"
	"github.com/gittuf/gittuf/internal/cmd/policy/persistent"
	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/repository"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/spf13/cobra"
)

type options struct {
	p            *persistent.Options
	policyName   string
	ruleName     string
	roleJSONFile string
	rulePatterns []string
	minRoles     int
}

type RoleJSON struct {
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
		&o.roleJSONFile,
		"roles-file",
		"",
		"path to role definition file",
	)

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

	var roleBytes []byte

	// We first see whether the user passed in the path to a JSON file or
	// piped the definitions into the CLI
	if o.roleJSONFile != "" {
		roleBytes, err = os.ReadFile(o.roleJSONFile)
		if err != nil {
			return err
		}
	} else {
		roleBytes, err = io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
	}

	// As both entry types are of the same syntax, we handle the byte array the
	// same way
	rolesToBeProcessed := []RoleJSON{}
	roles := []tuf.Role{}
	authorizedKeys := []*tuf.Key{}

	err = json.Unmarshal(roleBytes, &rolesToBeProcessed)
	if err != nil {
		return err
	}

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
		Use:               "add-rule-teams",
		Short:             "Add a new rule to a policy file with teams definitions",
		Long:              `This command allows users to add a new rule to the specified policy file using the teams functionality. By default, the main policy file is selected. Roles are specified in JSON either supplied via standard input (by default) or by a path to a JSON file. See the "teams-format.md" document in the docs directory for the required format.`,
		PreRunE:           common.CheckIfSigningViableWithFlag,
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
