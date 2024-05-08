// SPDX-License-Identifier: Apache-2.0

package listrules

import (
	"fmt"
	"strings"

	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/repository"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/sergi/go-diff/diffmatchpatch"
	"github.com/spf13/cobra"
)

type options struct{}

func (o *options) AddFlags(_ *cobra.Command) {}

func (o *options) Run(cmd *cobra.Command, _ []string) error {
	repo, err := repository.LoadRepository()
	if err != nil {
		return err
	}

	policyRules, policyRoot, err := repo.ListRules(cmd.Context(), policy.PolicyRef)
	if err != nil {
		return err
	}

	policyStagingRules, policyStagingRoot, err := repo.ListRules(cmd.Context(), policy.PolicyStagingRef)
	if err != nil {
		return err
	}

	fmt.Print(policy.GetDiffBetweenPolicyAndStaging(policyRules, policyStagingRules, policyRoot, policyStagingRoot))

	return nil
}

func FindDiffBetweenStrings(initial, withChanges string) string {
	dmp := diffmatchpatch.New()
	lines1, lines2, lineArray := dmp.DiffLinesToChars(initial, withChanges)
	diffs := dmp.DiffMain(lines1, lines2, false)
	diffs = dmp.DiffCharsToLines(diffs, lineArray)

	var diffDisplay strings.Builder

	for _, diff := range diffs {
		text := strings.TrimSuffix(diff.Text, "\n")
		lines := strings.Split(text, "\n")

		for _, line := range lines {
			switch diff.Type {
			case diffmatchpatch.DiffInsert:
				diffDisplay.WriteString(fmt.Sprintf("\033[32m+   %s\033[0m\n", line)) // Green for additions
			case diffmatchpatch.DiffDelete:
				diffDisplay.WriteString(fmt.Sprintf("\033[31m-   %s\033[0m\n", line)) // Red for deletions
			case diffmatchpatch.DiffEqual:
				diffDisplay.WriteString(fmt.Sprintf("    %s\n", line))
			}
		}
	}
	return diffDisplay.String()
}

func GetListRulesString(rule tuf.Delegation, depth int) string {
	var changes string

	changes += fmt.Sprintf(strings.Repeat("    ", depth)+"Rule %s:\n", rule.Name)
	gitpaths, filepaths := []string{}, []string{}
	for _, path := range rule.Paths {
		if strings.HasPrefix(path, "git:") {
			gitpaths = append(gitpaths, path)
		} else {
			filepaths = append(filepaths, path)
		}
	}
	if len(filepaths) > 0 {
		changes += fmt.Sprintf(strings.Repeat("    ", depth+1) + "Paths affected:" + "\n")
		for _, v := range filepaths {
			changes += fmt.Sprintf(strings.Repeat("    ", depth+2)+"%s\n", v)
		}
	}
	if len(gitpaths) > 0 {
		changes += fmt.Sprintf(strings.Repeat("    ", depth+1) + "Refs affected:" + "\n")
		for _, v := range gitpaths {
			changes += fmt.Sprintf(strings.Repeat("    ", depth+2)+"%s\n", v)
		}
	}

	changes += fmt.Sprintf(strings.Repeat("    ", depth+1) + "Authorized keys:" + "\n")
	for _, key := range rule.Role.KeyIDs {
		changes += fmt.Sprintf(strings.Repeat("    ", depth+2)+"%s\n", key)
	}

	changes += fmt.Sprintf(strings.Repeat("    ", depth+1) + fmt.Sprintf("Required valid signatures: %d", rule.Role.Threshold) + "\n")
	return changes
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:               "list-rules",
		Short:             "List rules for the current state",
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
