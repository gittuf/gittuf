// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gittuf/gittuf/experimental/gittuf"
)

// checkTransportExists returns true if the remote origin URL is already
// set to the transport.
func checkTransportExists(repo *gittuf.Repository) (bool, error) {
	url, err := repo.GetGitRepository().GetRemoteURL("origin")
	if err != nil {
		return false, err
	}
	return strings.HasPrefix(url, "gittuf::"), nil
}

// transportDoneMsg is sent when the final command completes.
type transportDoneMsg struct {
	steps []string
	err   error
}

// runTransportSetup creates a back up of the origin remote and rewrites
// origin remote to use the gittuf transport.
func runTransportSetup(repo *gittuf.Repository) tea.Cmd {
	return func() tea.Msg {
		time.Sleep(time.Second) // artificial delay for spinner visibility - can remove
		r := repo.GetGitRepository()
		var steps []string
		url, err := r.GetRemoteURL("origin")
		if err != nil {
			return transportDoneMsg{err: err}
		}
		err = r.AddRemote("origin-backup", url)
		if err != nil {
			return transportDoneMsg{err: err}
		}
		steps = append(steps, fmt.Sprintf("git remote add origin-backup %s", url))

		if err := r.RemoveRemote("origin"); err != nil {
			return transportDoneMsg{err: err}
		}
		err = r.AddRemote("origin", "gittuf::"+url)
		if err != nil {
			return transportDoneMsg{err: err}
		}
		steps = append(steps, fmt.Sprintf("git remote set-url origin gittuf::%s", url))
		return transportDoneMsg{steps: steps}
	}
}
