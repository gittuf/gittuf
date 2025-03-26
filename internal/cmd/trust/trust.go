// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package trust

import (
	"github.com/gittuf/gittuf/internal/cmd/trust/addcontrollerrepository"
	"github.com/gittuf/gittuf/internal/cmd/trust/addgithubapp"
	"github.com/gittuf/gittuf/internal/cmd/trust/addglobalrule"
	"github.com/gittuf/gittuf/internal/cmd/trust/addhook"
	"github.com/gittuf/gittuf/internal/cmd/trust/addnetworkrepository"
	"github.com/gittuf/gittuf/internal/cmd/trust/addpolicykey"
	"github.com/gittuf/gittuf/internal/cmd/trust/addpropagationdirective"
	"github.com/gittuf/gittuf/internal/cmd/trust/addrootkey"
	"github.com/gittuf/gittuf/internal/cmd/trust/disablegithubappapprovals"
	"github.com/gittuf/gittuf/internal/cmd/trust/enablegithubappapprovals"
	i "github.com/gittuf/gittuf/internal/cmd/trust/init"
	"github.com/gittuf/gittuf/internal/cmd/trust/listglobalrules"
	"github.com/gittuf/gittuf/internal/cmd/trust/listhooks"
	"github.com/gittuf/gittuf/internal/cmd/trust/makecontroller"
	"github.com/gittuf/gittuf/internal/cmd/trust/persistent"
	"github.com/gittuf/gittuf/internal/cmd/trust/removegithubapp"
	"github.com/gittuf/gittuf/internal/cmd/trust/removeglobalrule"
	"github.com/gittuf/gittuf/internal/cmd/trust/removehook"
	"github.com/gittuf/gittuf/internal/cmd/trust/removepolicykey"
	"github.com/gittuf/gittuf/internal/cmd/trust/removepropagationdirective"
	"github.com/gittuf/gittuf/internal/cmd/trust/removerootkey"
	"github.com/gittuf/gittuf/internal/cmd/trust/setrepositorylocation"
	"github.com/gittuf/gittuf/internal/cmd/trust/sign"
	"github.com/gittuf/gittuf/internal/cmd/trust/updateglobalrule"
	"github.com/gittuf/gittuf/internal/cmd/trust/updatepolicythreshold"
	"github.com/gittuf/gittuf/internal/cmd/trust/updaterootthreshold"
	"github.com/gittuf/gittuf/internal/cmd/trustpolicy/apply"
	"github.com/gittuf/gittuf/internal/cmd/trustpolicy/remote"
	"github.com/gittuf/gittuf/internal/cmd/trustpolicy/stage"
	"github.com/spf13/cobra"
)

func New() *cobra.Command {
	o := &persistent.Options{}
	cmd := &cobra.Command{
		Use:               "trust",
		Short:             "Tools for gittuf's root of trust",
		DisableAutoGenTag: true,
	}
	o.AddPersistentFlags(cmd)

	cmd.AddCommand(i.New(o))
	cmd.AddCommand(addcontrollerrepository.New(o))
	cmd.AddCommand(addgithubapp.New(o))
	cmd.AddCommand(addglobalrule.New(o))
	cmd.AddCommand(addhook.New(o))
	cmd.AddCommand(addnetworkrepository.New(o))
	cmd.AddCommand(addpolicykey.New(o))
	cmd.AddCommand(addpropagationdirective.New(o))
	cmd.AddCommand(addrootkey.New(o))
	cmd.AddCommand(apply.New())
	cmd.AddCommand(disablegithubappapprovals.New(o))
	cmd.AddCommand(enablegithubappapprovals.New(o))
	cmd.AddCommand(listhooks.New())
	cmd.AddCommand(makecontroller.New(o))
	cmd.AddCommand(remote.New())
	cmd.AddCommand(removegithubapp.New(o))
	cmd.AddCommand(removeglobalrule.New(o))
	cmd.AddCommand(removehook.New(o))
	cmd.AddCommand(removepolicykey.New(o))
	cmd.AddCommand(removepropagationdirective.New(o))
	cmd.AddCommand(removerootkey.New(o))
	cmd.AddCommand(setrepositorylocation.New(o))
	cmd.AddCommand(sign.New(o))
	cmd.AddCommand(stage.New())
	cmd.AddCommand(updateglobalrule.New(o))
	cmd.AddCommand(updatepolicythreshold.New(o))
	cmd.AddCommand(updaterootthreshold.New(o))
	cmd.AddCommand(listglobalrules.New())

	return cmd
}
