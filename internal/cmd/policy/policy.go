package policy

import (
	"github.com/adityasaky/gittuf/internal/cmd/policy/addgitbranchrule"
	"github.com/adityasaky/gittuf/internal/cmd/policy/addrule"
	i "github.com/adityasaky/gittuf/internal/cmd/policy/init"
	"github.com/adityasaky/gittuf/internal/cmd/policy/persistent"
	"github.com/adityasaky/gittuf/internal/cmd/policy/removerule"
	"github.com/spf13/cobra"
)

func New() *cobra.Command {
	o := &persistent.Options{}
	cmd := &cobra.Command{
		Use:   "policy",
		Short: "Tools to manage gittuf policies",
	}
	o.AddPersistentFlags(cmd)

	cmd.AddCommand(i.New(o))
	cmd.AddCommand(addrule.New(o))
	cmd.AddCommand(addgitbranchrule.New(o))
	cmd.AddCommand(removerule.New(o))

	return cmd
}
