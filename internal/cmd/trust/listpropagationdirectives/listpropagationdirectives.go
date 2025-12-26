// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package listpropagationdirectives

import (
	"fmt"

	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/spf13/cobra"
)

type options struct {
	targetRef string
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(
		&o.targetRef,
		"target-ref",
		"policy",
		"specify which policy ref should be inspected",
	)
}

func (o *options) Run(cmd *cobra.Command, _ []string) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	directives, err := repo.ListPropagationDirectives(cmd.Context(), o.targetRef)
	if err != nil {
		return err
	}
	// TODO: switch to the display package
	fmt.Println("Propagation Directives in the gittuf root of trust:")
	for _, pd := range directives {
		fmt.Printf("Propagation Directive: %s\n", pd.GetName())
		fmt.Printf("  Upstream Repository:   %s\n", pd.GetUpstreamRepository())
		fmt.Printf("  Upstream Reference:    %s\n", pd.GetUpstreamReference())
		fmt.Printf("  Upstream Path:         %s\n", pd.GetUpstreamPath())
		fmt.Printf("  Downstream Reference:  %s\n", pd.GetDownstreamReference())
		fmt.Printf("  Downstream Path:       %s\n", pd.GetDownstreamPath())
	}

	return nil
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:               "list-propagation-directives",
		Short:             "Lists propagation directives in the gittuf root of trust",
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
