// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"context"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gittuf/gittuf/experimental/gittuf"
	sslibdsse "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
)

type screen int

const (
	screenChoice screen = iota // initial screen
	screenMaintainerSelections
	screenTransport
	screenTransportConfirm
	screenAbort
	screenConclusion
)

type item struct {
	title, desc string
}

// Note: virtual methods must be implemented for the item struct
// Title returns the title of the item
func (i item) Title() string { return i.title }

// Description returns the description of the item
func (i item) Description() string { return i.desc }

// FilterValue returns the value to filter on
func (i item) FilterValue() string { return i.title }

type model struct {
	ctx              context.Context
	screen           screen
	choiceList       list.Model
	inputs           []textinput.Model
	focusIndex       int
	cursorMode       cursor.Mode
	repo             *gittuf.Repository
	signer           sslibdsse.SignerVerifier
	options          *options
	footer           string
	errorMsg         string
	rootExists       bool
	targetsExists    bool
	rootChoices      []string
	rootCursor       int
	rootSelected     map[int]bool
	transportExists  bool
	spinner          spinner.Model
	transportRunning bool
	transportSteps   []string
	width            int
}

// newDelegate creates a styled list delegate for use in all list.Model instances.
func newDelegate() list.DefaultDelegate {
	d := list.NewDefaultDelegate()
	d.Styles.SelectedTitle = selectedItemStyle
	d.Styles.SelectedDesc = selectedItemStyle
	d.Styles.NormalTitle = itemStyle
	d.Styles.NormalDesc = itemStyle
	return d
}

// newMenuList creates a configured list.Model with default settings.
func newMenuList(title string, items []list.Item, delegate list.DefaultDelegate) list.Model {
	l := list.New(items, delegate, 0, 0)
	l.Title = title
	l.Styles.Title = titleStyle
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	return l
}

// initialModel returns the initial model for the Terminal UI
func initialModel(ctx context.Context, o *options) (model, error) {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return model{}, err
	}

	delegate := newDelegate()

	s := spinner.New(spinner.WithSpinner(spinner.MiniDot))
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(colorFocus))

	m := model{
		ctx:        ctx,
		screen:     screenChoice,
		cursorMode: cursor.CursorBlink,
		repo:       repo,
		options:    o,
		choiceList: newMenuList("gittuf Setup", []list.Item{
			item{title: "Maintainer", desc: "I'm a maintainer"},
			item{title: "Contributor", desc: "I'm a contributor"},
		}, delegate),
		spinner:      s,
		rootChoices:  []string{"Make me a Root of Trust User", "Make me a Policy Administrator", "Authorize me to make changes to the default branch"},
		rootSelected: map[int]bool{addToRoot: true, addToTargets: true, addToRule: true}, // select all by default
	}

	signer, err := gittuf.LoadSignerFromGitConfig(repo)
	if err != nil {
		return model{}, err
	}
	m.signer = signer

	return m, nil
}

// Init initializes the input field
func (m model) Init() tea.Cmd {
	return textinput.Blink
}
