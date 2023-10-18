// SPDX-License-Identifier: Apache-2.0

package trust

import (
	"github.com/gittuf/gittuf/internal/cmd/trust/addpolicykey"
	i "github.com/gittuf/gittuf/internal/cmd/trust/init"
	"github.com/gittuf/gittuf/internal/cmd/trust/persistent"
	"github.com/gittuf/gittuf/internal/cmd/trust/removepolicykey"
	"github.com/gittuf/gittuf/internal/cmd/trustpolicy/remote"
	"github.com/spf13/cobra"
)

func New() *cobra.Command {
	o := &persistent.Options{}
	cmd := &cobra.Command{
		Use:   "trust",
		Short: "Tools for gittuf's root of trust",
	}
	o.AddPersistentFlags(cmd)

	cmd.AddCommand(i.New(o))
	cmd.AddCommand(addpolicykey.New(o))
	cmd.AddCommand(remote.New())
	cmd.AddCommand(removepolicykey.New(o))

	return cmd
}
