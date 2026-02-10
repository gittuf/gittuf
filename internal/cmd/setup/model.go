// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
)

type screen int

const (
	screenChoice screen = iota // initial screen
	screenAbout
	screenRoot
	screenTransport
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
	screen     screen
	choiceList list.Model
	inputs     []textinput.Model
	focusIndex int
	cursorMode cursor.Mode
	repo       *gittuf.Repository
	signer     dsse.SignerVerifier
	options    *options
	footer     string
}

// initialModel returns the initial model for the Terminal UI
func initialModel(o *options) (model, error) {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return model{}, err
	}

	// Initialize the model
	m := model{
		screen:     screenChoice,
		cursorMode: cursor.CursorBlink,
		repo:       repo,
		options:    o,
	}

	// Set up choice screen list items
	choiceItems := []list.Item{
		item{title: "Maintainer", desc: "I'm a maintainer"},
		item{title: "Contributer", desc: "I'm a contributer"},
	}

	// Set up the list delegate
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = selectedItemStyle
	delegate.Styles.SelectedDesc = selectedItemStyle
	delegate.Styles.NormalTitle = itemStyle
	delegate.Styles.NormalDesc = itemStyle

	// Set up choice screen list
	m.choiceList = list.New(choiceItems, delegate, 0, 0)
	m.choiceList.Title = "gittuf Setup"
	m.choiceList.SetShowStatusBar(false)
	m.choiceList.SetFilteringEnabled(false)
	m.choiceList.Styles.Title = titleStyle
	m.choiceList.SetShowHelp(false)

	return m, nil
}

// Init initializes the input field
func (m model) Init() tea.Cmd {
	return textinput.Blink
}
